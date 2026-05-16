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

func (l *exportLedger) IsReserved(busid string) bool {
	l.slow.Lock()
	defer l.slow.Unlock()
	return l.reservedLocked(busid)
}

// reservedLocked reports whether busid is unavailable for new admission:
// either marked busy by an active session or covered by an unexpired
// import lease. Caller must hold l.slow.
func (l *exportLedger) reservedLocked(busid string) bool {
	if l.busy[busid] {
		return true
	}
	lease, found := l.leases[busid]
	if !found {
		return false
	}
	return l.now().Before(lease.Expires)
}

func (l *exportLedger) AvailableExports() []Export {
	l.slow.Lock()
	out := make([]Export, 0, len(l.exports))
	for busid, export := range l.exports {
		if l.reservedLocked(busid) {
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
// confirms no goroutine raced in. An outstanding lease counts as busy so
// legacy OP_REQ_IMPORT cannot steal a slot a control client has already
// reserved via IssueLease. The caller must pair every success with a
// later ReleaseImport.
func (l *exportLedger) TryReserveForImport(ctx context.Context, busid string) (Export, bool, string) {
	l.slow.Lock()
	export, found := l.exports[busid]
	reserved := found && l.reservedLocked(busid)
	l.slow.Unlock()
	if !found {
		return nil, false, "unknown busid"
	}
	identity := export.LeaseIdentity()
	if reserved {
		return nil, false, deviceStateBusy
	}
	leaseOK, leaseReason := export.LeaseCheck(ctx)
	if !leaseOK {
		return nil, false, leaseReason
	}
	l.slow.Lock()
	defer l.slow.Unlock()
	current, stillExported := l.exports[busid]
	if !stillExported || current.LeaseIdentity() != identity {
		return nil, false, "unknown busid"
	}
	if l.reservedLocked(busid) {
		return nil, false, deviceStateBusy
	}
	l.busy[busid] = true
	return export, true, ""
}

func (l *exportLedger) ReleaseImport(ctx context.Context, busid string, removeExport bool) {
	l.mutateAndBroadcast(ctx, func() bool {
		delete(l.busy, busid)
		if removeExport {
			delete(l.exports, busid)
		}
		return true
	})
}

// mutateAndBroadcast holds slow for the body, then broadcasts iff body
// reported a change. This centralizes the "any reservedLocked-affecting
// mutation must broadcast" invariant so future mutation sites cannot
// silently skip it. body must NOT acquire fast (lock-ordering rule at
// the top of this file).
func (l *exportLedger) mutateAndBroadcast(ctx context.Context, body func() bool) {
	l.slow.Lock()
	changed := body()
	l.slow.Unlock()
	if changed {
		l.BroadcastIfChanged(ctx)
	}
}

// IssueLease captures the current broadcast sequence as opaque metadata
// for control clients. Lease correctness itself is pinned to the
// export's internal identity, not to that sequence number.
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

	identity := export.LeaseIdentity()
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
	if !stillExported || current.LeaseIdentity() != identity {
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
		Identity:     current.LeaseIdentity(),
		Expires:      now.Add(l.ttl),
	}
	l.leases[request.BusID] = lease
	response.LeaseID = lease.ID
	response.Generation = lease.Generation
	response.TTLMillis = int64(l.ttl / time.Millisecond)
	return response
}

// ConsumeLeaseAndReserve validates the requested lease against the
// current export identity, reruns LeaseCheck outside the slow lock, then
// atomically consumes the lease and marks the busid busy. Correctness is
// tied to the export identity rather than to the control sequence.
//
// Consume-on-read semantics from the old ConsumeLease are preserved:
// the lease entry is removed on every outcome except a nonce/ID
// mismatch (which preserves the lease for the legitimate holder).
// The caller must pair every success with a later ReleaseImport.
func (l *exportLedger) ConsumeLeaseAndReserve(ctx context.Context, request ImportExtRequest) (Export, bool, string) {
	var (
		export   Export
		identity ExportLeaseIdentity
		phase1OK bool
		reason   string
	)
	l.mutateAndBroadcast(ctx, func() bool {
		now := l.now()
		changed := l.cleanupExpiredLocked(now)

		lease, found := l.leases[request.BusID]
		if !found {
			reason = "lease not found"
			return changed
		}
		if lease.ID != request.LeaseID || lease.ClientNonce != request.ClientNonce {
			reason = "lease mismatch"
			return changed
		}
		if !now.Before(lease.Expires) {
			delete(l.leases, request.BusID)
			reason = "lease expired"
			return true
		}
		current, stillExported := l.exports[request.BusID]
		if !stillExported {
			delete(l.leases, request.BusID)
			reason = "unknown busid"
			return true
		}
		identity = current.LeaseIdentity()
		if identity != lease.Identity {
			delete(l.leases, request.BusID)
			reason = "lease stale"
			return true
		}
		if l.busy[request.BusID] {
			delete(l.leases, request.BusID)
			reason = deviceStateBusy
			return true
		}
		export = current
		phase1OK = true
		return changed
	})
	if !phase1OK {
		return nil, false, reason
	}

	leaseOK, leaseReason := export.LeaseCheck(ctx)
	if !leaseOK {
		l.mutateAndBroadcast(ctx, func() bool {
			currentLease, exists := l.leases[request.BusID]
			if !exists {
				return false
			}
			if currentLease.ID != request.LeaseID || currentLease.ClientNonce != request.ClientNonce {
				return false
			}
			delete(l.leases, request.BusID)
			return true
		})
		return nil, false, leaseReason
	}

	var (
		finalExport Export
		finalOK     bool
	)
	l.mutateAndBroadcast(ctx, func() bool {
		now := l.now()
		changed := l.cleanupExpiredLocked(now)

		lease, found := l.leases[request.BusID]
		if !found {
			reason = "lease not found"
			return changed
		}
		if lease.ID != request.LeaseID || lease.ClientNonce != request.ClientNonce {
			reason = "lease mismatch"
			return changed
		}
		if !now.Before(lease.Expires) {
			delete(l.leases, request.BusID)
			reason = "lease expired"
			return true
		}
		current, stillExported := l.exports[request.BusID]
		if !stillExported {
			delete(l.leases, request.BusID)
			reason = "unknown busid"
			return true
		}
		if lease.Identity != identity || current.LeaseIdentity() != identity {
			delete(l.leases, request.BusID)
			reason = "lease stale"
			return true
		}
		if l.busy[request.BusID] {
			delete(l.leases, request.BusID)
			reason = deviceStateBusy
			return true
		}
		delete(l.leases, request.BusID)
		l.busy[request.BusID] = true
		finalExport = current
		finalOK = true
		return true
	})
	if !finalOK {
		return nil, false, reason
	}
	return finalExport, true, ""
}

func (l *exportLedger) cleanupExpiredLocked(now time.Time) bool {
	changed := false
	for busid, lease := range l.leases {
		if !now.Before(lease.Expires) {
			delete(l.leases, busid)
			changed = true
		}
	}
	return changed
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
// reclaim; the transport read loop has already exited. Any leases the
// subscriber held are released and the resulting state change is
// broadcast so remaining subscribers see the busid become available
// again.
func (l *exportLedger) Unsubscribe(ctx context.Context, sub *exportSubscriber) {
	l.fast.Lock()
	delete(l.subs, sub.id)
	l.fast.Unlock()
	l.mutateAndBroadcast(ctx, func() bool {
		released := false
		for busid, lease := range l.leases {
			if lease.SubscriberID == sub.id {
				delete(l.leases, busid)
				released = true
			}
		}
		return released
	})
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
		entries = append(entries, entry{export: export, busy: l.reservedLocked(busid)})
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
		if snapshot.State == deviceStateUnavailable && snapshot.Entry.Info.BusIDString() == "" {
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
