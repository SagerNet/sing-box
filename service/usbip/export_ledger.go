//go:build linux || (darwin && cgo)

package usbip

import (
	"context"
	"net"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/log"
)

// exportLedger uses two internal mutexes that are NEVER held
// simultaneously. Each public method acquires at most one at a time;
// multi-stage methods acquire one, do unlocked work (including
// syscalls), then acquire the other.
//
//	fast: broadcast bookkeeping. seq, nextSubID, subs, state.
//	slow: inventory and leases. exports, busy, leases, nextLeaseID.
type exportLedger struct {
	logger log.ContextLogger
	now    func() time.Time
	ttl    time.Duration

	fast      sync.Mutex
	seq       uint64
	nextSubID uint64
	subs      map[uint64]*exportSubscriber
	state     map[string]DeviceInfoV2

	slow        sync.Mutex
	exports     map[string]Export
	busy        map[string]bool
	leases      map[string]serverImportLease
	nextLeaseID uint64
}

type exportSubscriber struct {
	id           uint64
	capabilities uint32
	conn         net.Conn
	send         chan controlMessage
}

const controlSubscriberSendBuffer = 16

func newExportLedger(logger log.ContextLogger, ttl time.Duration, now func() time.Time) *exportLedger {
	if now == nil {
		now = time.Now
	}
	return &exportLedger{
		logger:  logger,
		now:     now,
		ttl:     ttl,
		subs:    make(map[uint64]*exportSubscriber),
		state:   make(map[string]DeviceInfoV2),
		exports: make(map[string]Export),
		busy:    make(map[string]bool),
		leases:  make(map[string]serverImportLease),
	}
}

func (l *exportLedger) IsBusy(busid string) bool {
	l.slow.Lock()
	defer l.slow.Unlock()
	return l.busy[busid]
}

func (l *exportLedger) AvailableExports() []Export {
	l.slow.Lock()
	out := make([]Export, 0, len(l.exports))
	for busid, export := range l.exports {
		if l.busy[busid] {
			continue
		}
		out = append(out, export)
	}
	l.slow.Unlock()
	slices.SortFunc(out, func(a, b Export) int {
		return strings.Compare(a.BusID(), b.BusID())
	})
	return out
}

// ApplyHostSnapshot does not broadcast; callers pair it with
// SeedBroadcastState (quiet) or BroadcastIfChanged.
func (l *exportLedger) ApplyHostSnapshot(snapshot map[string]Export, released []string) {
	l.slow.Lock()
	l.exports = snapshot
	for _, busid := range released {
		delete(l.busy, busid)
	}
	l.slow.Unlock()
}

func (l *exportLedger) SeedBroadcastState(ctx context.Context) {
	nextState := deviceInfoV2Map(l.snapshotDeviceState(ctx))
	l.fast.Lock()
	l.state = nextState
	l.fast.Unlock()
}

func (l *exportLedger) BroadcastIfChanged(ctx context.Context) bool {
	nextState := deviceInfoV2Map(l.snapshotDeviceState(ctx))

	l.fast.Lock()
	nextSequence := l.seq + 1
	delta := buildControlDeviceDelta(nextSequence, l.state, nextState)
	if len(delta.Added) == 0 && len(delta.Updated) == 0 && len(delta.Removed) == 0 {
		l.state = nextState
		l.fast.Unlock()
		return false
	}
	l.seq = nextSequence
	sequence := l.seq
	l.state = nextState
	targets := make([]*exportSubscriber, 0, len(l.subs))
	for _, sub := range l.subs {
		targets = append(targets, sub)
	}
	l.fast.Unlock()

	frame := controlFrame{
		Type:     controlFrameChanged,
		Version:  controlProtocolVersion,
		Sequence: sequence,
	}
	for _, sub := range targets {
		if supportsControlExtensions(sub.capabilities) {
			l.enqueuePayload(sub, controlFrame{
				Type:     controlFrameDeviceDelta,
				Version:  controlProtocolVersion,
				Sequence: sequence,
			}, delta, frame)
			continue
		}
		l.enqueueFrame(sub, frame)
	}
	return true
}

// TryReserveForImport runs Export.LeaseCheck outside the slow lock; the
// busy mark is inserted only after a second availability re-check
// confirms no goroutine raced in. The caller must pair every success
// with a later ReleaseImport.
func (l *exportLedger) TryReserveForImport(ctx context.Context, busid string) (Export, bool, string) {
	l.slow.Lock()
	export, found := l.exports[busid]
	busy := l.busy[busid]
	l.slow.Unlock()
	if !found {
		return nil, false, "unknown busid"
	}
	if busy {
		return nil, false, deviceStateBusy
	}
	leaseOK, leaseReason := export.LeaseCheck(ctx)
	if !leaseOK {
		return nil, false, leaseReason
	}
	l.slow.Lock()
	defer l.slow.Unlock()
	current, stillExported := l.exports[busid]
	if !stillExported || current != export {
		return nil, false, "unknown busid"
	}
	if l.busy[busid] {
		return nil, false, deviceStateBusy
	}
	l.busy[busid] = true
	return export, true, ""
}

func (l *exportLedger) ReleaseImport(ctx context.Context, busid string, removeExport bool) {
	l.slow.Lock()
	delete(l.busy, busid)
	if removeExport {
		delete(l.exports, busid)
	}
	l.slow.Unlock()
	l.BroadcastIfChanged(ctx)
}

// IssueLease captures the seq generation at entry so a subsequent
// ConsumeLease can reject stale leases issued before a topology change.
func (l *exportLedger) IssueLease(ctx context.Context, subID uint64, request controlLeaseRequest) controlLeaseResponse {
	response := controlLeaseResponse{
		BusID:       request.BusID,
		ClientNonce: request.ClientNonce,
	}
	if request.BusID == "" {
		response.ErrorCode = leaseErrorBadRequest
		response.ErrorMessage = "missing busid"
		return response
	}

	l.fast.Lock()
	generation := l.seq
	l.fast.Unlock()

	l.slow.Lock()
	now := l.now()
	l.cleanupExpiredLocked(now)
	export, found := l.exports[request.BusID]
	if !found {
		l.slow.Unlock()
		response.ErrorCode = leaseErrorUnavailable
		response.ErrorMessage = "unknown busid"
		return response
	}
	if l.busy[request.BusID] {
		l.slow.Unlock()
		response.ErrorCode = leaseErrorUnavailable
		response.ErrorMessage = deviceStateBusy
		return response
	}
	if _, exists := l.leases[request.BusID]; exists {
		l.slow.Unlock()
		response.ErrorCode = leaseErrorBusy
		response.ErrorMessage = "lease already active"
		return response
	}
	l.slow.Unlock()

	leaseOK, leaseReason := export.LeaseCheck(ctx)
	if !leaseOK {
		response.ErrorCode = leaseErrorUnavailable
		response.ErrorMessage = leaseReason
		return response
	}

	l.slow.Lock()
	defer l.slow.Unlock()
	now = l.now()
	l.cleanupExpiredLocked(now)
	current, stillExported := l.exports[request.BusID]
	if !stillExported || current != export {
		response.ErrorCode = leaseErrorUnavailable
		response.ErrorMessage = "unknown busid"
		return response
	}
	if l.busy[request.BusID] {
		response.ErrorCode = leaseErrorUnavailable
		response.ErrorMessage = deviceStateBusy
		return response
	}
	if _, exists := l.leases[request.BusID]; exists {
		response.ErrorCode = leaseErrorBusy
		response.ErrorMessage = "lease already active"
		return response
	}
	l.nextLeaseID++
	lease := serverImportLease{
		ID:           l.nextLeaseID,
		SubscriberID: subID,
		BusID:        request.BusID,
		ClientNonce:  request.ClientNonce,
		Generation:   generation,
		Expires:      now.Add(l.ttl),
	}
	l.leases[request.BusID] = lease
	response.LeaseID = lease.ID
	response.Generation = lease.Generation
	response.TTLMillis = int64(l.ttl / time.Millisecond)
	return response
}

// ConsumeLease has consume-on-read semantics: the entry is removed
// regardless of outcome, except on mismatched nonce — which preserves
// the lease for the legitimate holder.
func (l *exportLedger) ConsumeLease(request ImportExtRequest) bool {
	l.slow.Lock()
	now := l.now()
	l.cleanupExpiredLocked(now)
	lease, found := l.leases[request.BusID]
	if !found {
		l.slow.Unlock()
		return false
	}
	if lease.ID != request.LeaseID || lease.ClientNonce != request.ClientNonce {
		l.slow.Unlock()
		return false
	}
	delete(l.leases, request.BusID)
	leaseExpiry := lease.Expires
	leaseGeneration := lease.Generation
	l.slow.Unlock()
	if !now.Before(leaseExpiry) {
		return false
	}
	l.fast.Lock()
	currentGeneration := l.seq
	l.fast.Unlock()
	return leaseGeneration == currentGeneration
}

func (l *exportLedger) cleanupExpiredLocked(now time.Time) {
	for busid, lease := range l.leases {
		if !now.Before(lease.Expires) {
			delete(l.leases, busid)
		}
	}
}

// Subscribe enqueues a freshly computed snapshot to extension-capable
// subscribers so they see current state regardless of when the last
// broadcast fired. Does NOT mutate l.state: other subscribers must
// still receive the next BroadcastIfChanged delta against the previous
// baseline.
func (l *exportLedger) Subscribe(ctx context.Context, conn net.Conn, capabilities uint32) (*exportSubscriber, uint64) {
	extended := supportsControlExtensions(capabilities)
	var snapshot []DeviceInfoV2
	var sequence uint64
	if extended {
		// Keep the snapshot and sequence from the same stable generation.
		for {
			l.fast.Lock()
			sequence = l.seq
			l.fast.Unlock()

			snapshot = l.snapshotDeviceState(ctx)

			l.fast.Lock()
			if sequence == l.seq {
				break
			}
			l.fast.Unlock()
		}
	} else {
		l.fast.Lock()
		sequence = l.seq
	}
	defer l.fast.Unlock()
	l.nextSubID++
	sub := &exportSubscriber{
		id:           l.nextSubID,
		capabilities: capabilities,
		conn:         conn,
		send:         make(chan controlMessage, controlSubscriberSendBuffer),
	}
	if extended {
		l.enqueuePayload(sub, controlFrame{
			Type:     controlFrameDeviceSnapshot,
			Version:  controlProtocolVersion,
			Sequence: sequence,
		}, controlDeviceSnapshot{Sequence: sequence, Devices: snapshot}, controlFrame{
			Type:     controlFrameChanged,
			Version:  controlProtocolVersion,
			Sequence: sequence,
		})
	}
	l.subs[sub.id] = sub
	return sub, sequence
}

// Unsubscribe leaves the subscriber's send channel for the GC to
// reclaim; the transport read loop has already exited.
func (l *exportLedger) Unsubscribe(sub *exportSubscriber) {
	l.fast.Lock()
	delete(l.subs, sub.id)
	l.fast.Unlock()
	l.slow.Lock()
	for busid, lease := range l.leases {
		if lease.SubscriberID == sub.id {
			delete(l.leases, busid)
		}
	}
	l.slow.Unlock()
}

// CloseAllSubscribers returns the underlying connections so the caller
// can close them outside any lock.
func (l *exportLedger) CloseAllSubscribers() []net.Conn {
	l.fast.Lock()
	conns := make([]net.Conn, 0, len(l.subs))
	for _, sub := range l.subs {
		conns = append(conns, sub.conn)
	}
	l.subs = make(map[uint64]*exportSubscriber)
	l.fast.Unlock()
	return conns
}

func (l *exportLedger) ResetForClose() {
	l.slow.Lock()
	l.exports = make(map[string]Export)
	l.busy = make(map[string]bool)
	l.leases = make(map[string]serverImportLease)
	l.slow.Unlock()
}

func (l *exportLedger) HandleControlLeaseRequest(ctx context.Context, sub *exportSubscriber, payload []byte) {
	var request controlLeaseRequest
	err := unmarshalControlPayload(payload, &request)
	if err != nil {
		l.fast.Lock()
		sequence := l.seq
		l.fast.Unlock()
		l.enqueuePayload(sub, controlFrame{
			Type:    controlFrameLeaseResponse,
			Version: controlProtocolVersion,
		}, controlLeaseResponse{
			ErrorCode:    leaseErrorBadRequest,
			ErrorMessage: err.Error(),
		}, controlFrame{Type: controlFrameChanged, Version: controlProtocolVersion, Sequence: sequence})
		return
	}
	response := l.IssueLease(ctx, sub.id, request)
	l.fast.Lock()
	sequence := l.seq
	l.fast.Unlock()
	l.enqueuePayload(sub, controlFrame{
		Type:    controlFrameLeaseResponse,
		Version: controlProtocolVersion,
	}, response, controlFrame{Type: controlFrameChanged, Version: controlProtocolVersion, Sequence: sequence})
}

func (l *exportLedger) snapshotDeviceState(ctx context.Context) []DeviceInfoV2 {
	type entry struct {
		export Export
		busy   bool
	}
	l.slow.Lock()
	entries := make([]entry, 0, len(l.exports))
	for busid, export := range l.exports {
		entries = append(entries, entry{export: export, busy: l.busy[busid]})
	}
	l.slow.Unlock()
	if len(entries) == 0 {
		return nil
	}
	slices.SortFunc(entries, func(a, b entry) int {
		return strings.Compare(a.export.BusID(), b.export.BusID())
	})
	out := make([]DeviceInfoV2, 0, len(entries))
	for _, e := range entries {
		snapshot := e.export.Snapshot(ctx, e.busy)
		if snapshot.State == deviceStateUnavailable && snapshot.Entry.Info.IDVendor == 0 {
			continue
		}
		out = append(out, deviceInfoV2FromEntry(snapshot.Entry, snapshot.Backend, snapshot.StableID, snapshot.State, snapshot.RawStatus, snapshot.StatusReason))
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

func (l *exportLedger) enqueuePayload(sub *exportSubscriber, frame controlFrame, payload any, fallback controlFrame) {
	rawPayload, err := marshalControlPayload(payload)
	if err != nil || len(rawPayload) > maxControlPayloadLength {
		l.enqueueFrame(sub, fallback)
		return
	}
	select {
	case sub.send <- controlMessage{Frame: frame, Payload: rawPayload}:
	default:
		l.logger.Debug("control subscriber ", sub.id, " lagged behind")
		_ = sub.conn.Close()
	}
}
