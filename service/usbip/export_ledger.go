//go:build linux || (darwin && cgo) || windows

package usbip

import (
	"maps"
	"net"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/log"
)

// exportLedger lock order: broadcastAccess may be taken first and then
// inventoryAccess nested inside (BroadcastIfChanged computes the next
// state via snapshotDeviceState while holding broadcastAccess, so that
// compute and publish are one atomic step — two racing broadcasts can
// otherwise publish out of order and lock in a stale snapshot). Code
// holding inventoryAccess must never take broadcastAccess.
type exportLedger struct {
	logger log.ContextLogger
	now    func() time.Time

	broadcastAccess sync.Mutex
	nextSubID       uint64
	subs            map[uint64]*exportSubscriber
	state           map[string]ControlDeviceInfo

	inventoryAccess sync.Mutex
	exports         map[string]Export
	busy            map[string]bool
}

type exportSubscriber struct {
	id   uint64
	conn net.Conn
	send chan controlMessage
}

const controlSubscriberSendBuffer = 16

func newExportLedger(logger log.ContextLogger, now func() time.Time) *exportLedger {
	if now == nil {
		now = time.Now
	}
	return &exportLedger{
		logger:  logger,
		now:     now,
		subs:    make(map[uint64]*exportSubscriber),
		state:   make(map[string]ControlDeviceInfo),
		exports: make(map[string]Export),
		busy:    make(map[string]bool),
	}
}

// withInventoryWrite broadcasts iff body returns true. body must not
// acquire the broadcast lock.
func (l *exportLedger) withInventoryWrite(body func() bool) {
	l.inventoryAccess.Lock()
	changed := body()
	l.inventoryAccess.Unlock()
	if changed {
		l.BroadcastIfChanged()
	}
}

func (l *exportLedger) withInventoryRead(body func()) {
	l.inventoryAccess.Lock()
	defer l.inventoryAccess.Unlock()
	body()
}

func (l *exportLedger) withInventoryWriteQuiet(body func()) {
	l.inventoryAccess.Lock()
	defer l.inventoryAccess.Unlock()
	body()
}

func (l *exportLedger) IsReserved(busid string) bool {
	var reserved bool
	l.withInventoryRead(func() {
		reserved = l.busy[busid]
	})
	return reserved
}

func (l *exportLedger) AvailableExports() []Export {
	var out []Export
	l.withInventoryRead(func() {
		out = make([]Export, 0, len(l.exports))
		for busid, export := range l.exports {
			if l.busy[busid] {
				continue
			}
			out = append(out, export)
		}
	})
	slices.SortFunc(out, func(a, b Export) int {
		return strings.Compare(a.BusID(), b.BusID())
	})
	return out
}

// ApplyHostSnapshot does not broadcast; callers pair it with
// SeedBroadcastState (quiet) or BroadcastIfChanged.
func (l *exportLedger) ApplyHostSnapshot(snapshot map[string]Export, released []string) {
	l.withInventoryWriteQuiet(func() {
		l.exports = snapshot
		for _, busid := range released {
			delete(l.busy, busid)
		}
	})
}

func (l *exportLedger) SeedBroadcastState() {
	l.broadcastAccess.Lock()
	l.state = controlDeviceInfoMap(l.snapshotDeviceState())
	l.broadcastAccess.Unlock()
}

// BroadcastIfChanged computes, publishes, and enqueues the snapshot in
// one broadcastAccess critical section. Enqueueing outside the lock
// would let two racing broadcasts deliver in the opposite order they
// published, leaving subscribers on the stale snapshot. Enqueue is a
// non-blocking channel send, so holding the lock across it is cheap.
func (l *exportLedger) BroadcastIfChanged() bool {
	l.broadcastAccess.Lock()
	defer l.broadcastAccess.Unlock()
	nextState := controlDeviceInfoMap(l.snapshotDeviceState())
	if maps.EqualFunc(l.state, nextState, controlDeviceInfoEqual) {
		return false
	}
	l.state = nextState
	frame := controlFrame{Type: controlFrameDeviceSnapshot, Version: controlProtocolVersion}
	payload := controlDeviceSnapshot{Devices: sortedControlDeviceInfoValues(nextState)}
	for _, sub := range l.subs {
		l.enqueuePayload(sub, frame, payload)
	}
	return true
}

// The single critical section closes the TOCTOU window between
// availability check and busy mark. Caller must pair success with
// ReleaseImport and broadcast once the session is wired up.
func (l *exportLedger) TryReserveForImport(busid string) (Export, bool, string) {
	var (
		export Export
		ok     bool
		reason string
	)
	l.withInventoryWriteQuiet(func() {
		current, found := l.exports[busid]
		if !found {
			reason = "unknown busid"
			return
		}
		if l.busy[busid] {
			reason = deviceStateBusy
			return
		}
		l.busy[busid] = true
		export = current
		ok = true
	})
	if !ok {
		return nil, false, reason
	}
	return export, true, ""
}

func (l *exportLedger) ReleaseImport(busid string, removeExport bool) {
	l.withInventoryWrite(func() bool {
		delete(l.busy, busid)
		if removeExport {
			delete(l.exports, busid)
		}
		return true
	})
}

// Subscribe enqueues the current broadcast state so the new subscriber
// sees the same device list every existing subscriber has received.
// l.state is maintained by SeedBroadcastState and BroadcastIfChanged
// under broadcastAccess, so reading it here is race-free.
func (l *exportLedger) Subscribe(conn net.Conn) *exportSubscriber {
	l.broadcastAccess.Lock()
	defer l.broadcastAccess.Unlock()
	l.nextSubID++
	sub := &exportSubscriber{
		id:   l.nextSubID,
		conn: conn,
		send: make(chan controlMessage, controlSubscriberSendBuffer),
	}
	l.enqueuePayload(sub, controlFrame{
		Type:    controlFrameDeviceSnapshot,
		Version: controlProtocolVersion,
	}, controlDeviceSnapshot{Devices: sortedControlDeviceInfoValues(l.state)})
	l.subs[sub.id] = sub
	return sub
}

func (l *exportLedger) Unsubscribe(sub *exportSubscriber) {
	l.broadcastAccess.Lock()
	delete(l.subs, sub.id)
	l.broadcastAccess.Unlock()
}

// Returns connections to close outside the lock.
func (l *exportLedger) CloseAllSubscribers() []net.Conn {
	l.broadcastAccess.Lock()
	conns := make([]net.Conn, 0, len(l.subs))
	for _, sub := range l.subs {
		conns = append(conns, sub.conn)
	}
	l.subs = make(map[uint64]*exportSubscriber)
	l.broadcastAccess.Unlock()
	return conns
}

func (l *exportLedger) ResetForClose() {
	l.withInventoryWriteQuiet(func() {
		l.exports = make(map[string]Export)
		l.busy = make(map[string]bool)
	})
}

func (l *exportLedger) snapshotDeviceState() []ControlDeviceInfo {
	type entry struct {
		export Export
		busy   bool
	}
	var entries []entry
	l.withInventoryRead(func() {
		entries = make([]entry, 0, len(l.exports))
		for busid, export := range l.exports {
			entries = append(entries, entry{export: export, busy: l.busy[busid]})
		}
	})
	if len(entries) == 0 {
		return nil
	}
	slices.SortFunc(entries, func(a, b entry) int {
		return strings.Compare(a.export.BusID(), b.export.BusID())
	})
	out := make([]ControlDeviceInfo, 0, len(entries))
	for _, e := range entries {
		snapshot := e.export.Snapshot(e.busy)
		if snapshot.State == deviceStateUnavailable && snapshot.Entry.Info.BusIDString() == "" {
			continue
		}
		out = append(out, controlDeviceInfoFromEntry(snapshot.Entry, snapshot.Backend, snapshot.StableID, snapshot.State, snapshot.RawStatus, snapshot.StatusReason))
	}
	return out
}

func (l *exportLedger) enqueueFrame(sub *exportSubscriber, frame controlFrame) {
	select {
	case sub.send <- controlMessage{Frame: frame}:
	default:
		l.logger.Debug("control subscriber ", sub.id, " lagged behind")
		_ = sub.conn.Close()
	}
}

func (l *exportLedger) enqueuePayload(sub *exportSubscriber, frame controlFrame, payload any) {
	rawPayload, err := marshalControlPayload(payload)
	if err != nil || len(rawPayload) > maxControlPayloadLength {
		l.logger.Debug("control subscriber ", sub.id, " payload encode failed; closing")
		_ = sub.conn.Close()
		return
	}
	select {
	case sub.send <- controlMessage{Frame: frame, Payload: rawPayload}:
	default:
		l.logger.Debug("control subscriber ", sub.id, " lagged behind")
		_ = sub.conn.Close()
	}
}
