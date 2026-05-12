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

// exportLedger owns the server's authoritative mutable state: which
// devices it is exporting, which busids are currently in use, who is
// subscribed to control-channel updates, and which import leases are
// outstanding. It absorbs what previously lived as four fields plus a
// LeaseManager on ServerService.
//
// Synchronization:
//
//	The ledger uses two internal mutexes. The two are NEVER held
//	simultaneously. Each public method acquires at most one at a time
//	for any nested duration; multi-stage methods acquire and release
//	one, do unlocked work (including syscalls), then acquire the
//	other.
//
//	fast: broadcast bookkeeping. seq, nextSubID, subs, state.
//	slow: inventory and leases. exports, busy, leases, nextLeaseID.
//
//	The previous controlAccess -> LeaseManager.access -> access
//	ordering documented in lease.go disappears because the only
//	remaining cross-state operation (IssueLease) reads seq under fast,
//	releases, then uses slow.
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

// exportSubscriber is one live control-channel connection. The send
// channel is filled by the ledger's broadcast methods and drained by
// the transport handler.
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

// IsBusy reports whether the busid currently has an active import. The
// signature matches ExportHost.Reconcile's isBusy callback.
func (l *exportLedger) IsBusy(busid string) bool {
	l.slow.Lock()
	defer l.slow.Unlock()
	return l.busy[busid]
}

// Export returns the export registered for busid, if any.
func (l *exportLedger) Export(busid string) (Export, bool) {
	l.slow.Lock()
	defer l.slow.Unlock()
	export, found := l.exports[busid]
	return export, found
}

// AvailableExports returns exports that are not currently busy, sorted
// by busid. The slice references are stable; Snapshot may be called on
// each entry outside the ledger's lock.
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
	slices.SortFunc(out, exportLess)
	return out
}

func exportLess(a, b Export) int {
	return strings.Compare(a.BusID(), b.BusID())
}

// ApplyHostSnapshot replaces the inventory with snapshot and clears
// busy entries for released busids. Does not broadcast — callers pair
// this with SeedBroadcastState (quiet) or BroadcastIfChanged.
func (l *exportLedger) ApplyHostSnapshot(snapshot map[string]Export, released []string) {
	l.slow.Lock()
	l.exports = snapshot
	for _, busid := range released {
		delete(l.busy, busid)
	}
	l.slow.Unlock()
}

// SeedBroadcastState recomputes the broadcast state and stores it
// without emitting any frame. Used at Start.
func (l *exportLedger) SeedBroadcastState(ctx context.Context) {
	nextState := deviceInfoV2Map(l.snapshotDeviceState(ctx))
	l.fast.Lock()
	l.state = nextState
	l.fast.Unlock()
}

// BroadcastIfChanged recomputes the broadcast state, compares against
// the last broadcast, and emits a DeviceDelta + Changed frame to every
// subscriber if the state moved. Returns true if a frame was sent.
func (l *exportLedger) BroadcastIfChanged(ctx context.Context) bool {
	nextState := deviceInfoV2Map(l.snapshotDeviceState(ctx))

	l.fast.Lock()
	nextSequence := l.seq + 1
	delta := buildControlDeviceDelta(nextSequence, l.state, nextState)
	if controlDeviceDeltaEmpty(delta) {
		l.state = nextState
		l.fast.Unlock()
		return false
	}
	l.seq = nextSequence
	sequence := l.seq
	l.state = nextState
	targets := l.snapshotSubsLocked()
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

// TryReserveForImport atomically checks that busid is exported, not
// busy, and currently lease-available, marking it busy on success. The
// caller is responsible for follow-up DeviceInfo/NewServerDataSession
// and must call ReleaseImport on any failure path or ConfirmImport on
// success.
//
// The Export.LeaseCheck syscall runs outside the slow lock; the busy
// mark is only inserted after the lease check passes and the second
// availability re-check confirms no other goroutine raced in.
func (l *exportLedger) TryReserveForImport(ctx context.Context, busid string) (Export, bool, string) {
	export, reason, ok := l.checkAvailableUnderLock(busid)
	if !ok {
		return nil, false, reason
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

func (l *exportLedger) checkAvailableUnderLock(busid string) (Export, string, bool) {
	l.slow.Lock()
	defer l.slow.Unlock()
	export, found := l.exports[busid]
	if !found {
		return nil, "unknown busid", false
	}
	if l.busy[busid] {
		return nil, deviceStateBusy, false
	}
	return export, "", true
}

// ConfirmImport broadcasts that an import is now active. The busy mark
// was set during TryReserveForImport; this method only broadcasts so
// clients see the state change.
func (l *exportLedger) ConfirmImport(ctx context.Context) {
	l.BroadcastIfChanged(ctx)
}

// ReleaseImport clears the busy mark for busid, optionally removing
// the export entirely, and broadcasts the change. removeExport=true is
// used when the platform host's FinishImport returns released=true
// (e.g. Darwin stale capture).
func (l *exportLedger) ReleaseImport(ctx context.Context, busid string, removeExport bool) {
	l.slow.Lock()
	delete(l.busy, busid)
	if removeExport {
		delete(l.exports, busid)
	}
	l.slow.Unlock()
	l.BroadcastIfChanged(ctx)
}

// IssueLease validates the request and inserts a fresh lease keyed by
// busid. The seq generation is captured at entry and stored on the
// lease so a subsequent ConsumeLease can reject stale leases issued
// before a topology change.
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

	export, errorCode, reason, ok := l.checkAvailableForLease(request.BusID)
	if !ok {
		response.ErrorCode = errorCode
		response.ErrorMessage = reason
		return response
	}
	leaseOK, leaseReason := export.LeaseCheck(ctx)
	if !leaseOK {
		response.ErrorCode = leaseErrorUnavailable
		response.ErrorMessage = leaseReason
		return response
	}

	l.slow.Lock()
	defer l.slow.Unlock()
	now := l.now()
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

func (l *exportLedger) checkAvailableForLease(busid string) (Export, string, string, bool) {
	l.slow.Lock()
	defer l.slow.Unlock()
	now := l.now()
	l.cleanupExpiredLocked(now)
	export, found := l.exports[busid]
	if !found {
		return nil, leaseErrorUnavailable, "unknown busid", false
	}
	if l.busy[busid] {
		return nil, leaseErrorUnavailable, deviceStateBusy, false
	}
	if _, exists := l.leases[busid]; exists {
		return nil, leaseErrorBusy, "lease already active", false
	}
	return export, "", "", true
}

// ConsumeLease validates and removes the lease matching request, then
// checks the lease's generation against the current seq. Returns true
// only if all checks pass. Consume-on-read semantics: the entry is
// removed regardless of outcome (except on mismatched nonce, which
// preserves the lease for the legitimate holder).
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

// Subscribe registers a new control-channel connection. If
// capabilities include the device-snapshot extension, a freshly
// computed snapshot is enqueued so the new subscriber sees current
// state regardless of when the last broadcast fired. Returns the
// subscriber and the current sequence.
//
// Does NOT mutate l.state: other subscribers must still receive the
// next BroadcastIfChanged delta against the previous baseline.
func (l *exportLedger) Subscribe(ctx context.Context, conn net.Conn, capabilities uint32) (*exportSubscriber, uint64) {
	snapshot := l.snapshotDeviceState(ctx)
	l.fast.Lock()
	defer l.fast.Unlock()
	l.nextSubID++
	sub := &exportSubscriber{
		id:           l.nextSubID,
		capabilities: capabilities,
		conn:         conn,
		send:         make(chan controlMessage, controlSubscriberSendBuffer),
	}
	sequence := l.seq
	if supportsControlExtensions(capabilities) {
		l.enqueueSnapshotLocked(sub, sequence, snapshot)
	}
	l.subs[sub.id] = sub
	return sub, sequence
}

// Unsubscribe removes a control connection and revokes any outstanding
// leases owned by it. The subscriber's send channel is left for the GC
// to reclaim — the transport read loop already exited.
func (l *exportLedger) Unsubscribe(sub *exportSubscriber) {
	l.fast.Lock()
	delete(l.subs, sub.id)
	l.fast.Unlock()
	l.revokeLeasesForSubscriber(sub.id)
}

func (l *exportLedger) revokeLeasesForSubscriber(subID uint64) {
	l.slow.Lock()
	defer l.slow.Unlock()
	for busid, lease := range l.leases {
		if lease.SubscriberID == subID {
			delete(l.leases, busid)
		}
	}
}

// CloseAllSubscribers drains every subscriber and returns the
// underlying connections so the caller can close them outside any
// lock.
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

// ResetForClose clears inventory and busy state. Called from Close
// after the host has been shut down.
func (l *exportLedger) ResetForClose() {
	l.slow.Lock()
	l.exports = make(map[string]Export)
	l.busy = make(map[string]bool)
	l.leases = make(map[string]serverImportLease)
	l.slow.Unlock()
}

// CurrentSequence returns the last broadcast sequence (for transport
// code that writes the initial ack frame).
func (l *exportLedger) CurrentSequence() uint64 {
	l.fast.Lock()
	defer l.fast.Unlock()
	return l.seq
}

// HandleControlPing enqueues a Pong response on sub.
func (l *exportLedger) HandleControlPing(sub *exportSubscriber) {
	l.enqueueFrame(sub, controlFrame{
		Type:    controlFramePong,
		Version: controlProtocolVersion,
	})
}

// HandleControlLeaseRequest parses payload, issues a lease, and
// enqueues the resulting LeaseResponse (with a Changed fallback if the
// payload exceeds maxControlPayloadLength).
func (l *exportLedger) HandleControlLeaseRequest(ctx context.Context, sub *exportSubscriber, payload []byte) {
	var request controlLeaseRequest
	err := unmarshalControlPayload(payload, &request)
	if err != nil {
		l.enqueuePayload(sub, controlFrame{
			Type:    controlFrameLeaseResponse,
			Version: controlProtocolVersion,
		}, controlLeaseResponse{
			ErrorCode:    leaseErrorBadRequest,
			ErrorMessage: err.Error(),
		}, controlFrame{Type: controlFrameChanged, Version: controlProtocolVersion, Sequence: l.CurrentSequence()})
		return
	}
	response := l.IssueLease(ctx, sub.id, request)
	l.enqueuePayload(sub, controlFrame{
		Type:    controlFrameLeaseResponse,
		Version: controlProtocolVersion,
	}, response, controlFrame{Type: controlFrameChanged, Version: controlProtocolVersion, Sequence: l.CurrentSequence()})
}

// snapshotDeviceState gathers refs under slow, releases, then calls
// Export.Snapshot for each entry outside the lock. The busy value
// captured at copy time is what's reported to clients.
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
		if snapshot.Err != nil {
			out = append(out, DeviceInfoV2{
				BusID:        e.export.BusID(),
				Backend:      snapshot.Backend,
				StableID:     snapshot.StableID,
				State:        deviceStateUnavailable,
				StatusReason: snapshot.StatusReason,
			})
			continue
		}
		if snapshot.State == deviceStateUnavailable && snapshot.Entry.Info.IDVendor == 0 {
			continue
		}
		out = append(out, deviceInfoV2FromEntry(snapshot.Entry, snapshot.Backend, snapshot.StableID, snapshot.State, snapshot.RawStatus, snapshot.StatusReason))
	}
	return out
}

func (l *exportLedger) snapshotSubsLocked() []*exportSubscriber {
	out := make([]*exportSubscriber, 0, len(l.subs))
	for _, sub := range l.subs {
		out = append(out, sub)
	}
	return out
}

func (l *exportLedger) enqueueSnapshotLocked(sub *exportSubscriber, sequence uint64, devices []DeviceInfoV2) {
	l.enqueuePayload(sub, controlFrame{
		Type:     controlFrameDeviceSnapshot,
		Version:  controlProtocolVersion,
		Sequence: sequence,
	}, controlDeviceSnapshot{Sequence: sequence, Devices: devices}, controlFrame{
		Type:     controlFrameChanged,
		Version:  controlProtocolVersion,
		Sequence: sequence,
	})
}

func (l *exportLedger) enqueueFrame(sub *exportSubscriber, frame controlFrame) {
	l.deliver(sub, controlMessage{Frame: frame})
}

func (l *exportLedger) enqueuePayload(sub *exportSubscriber, frame controlFrame, payload any, fallback controlFrame) {
	rawPayload, err := marshalControlPayload(payload)
	if err != nil || len(rawPayload) > maxControlPayloadLength {
		l.enqueueFrame(sub, fallback)
		return
	}
	l.deliver(sub, controlMessage{Frame: frame, Payload: rawPayload})
}

func (l *exportLedger) deliver(sub *exportSubscriber, message controlMessage) {
	select {
	case sub.send <- message:
	default:
		l.logger.Debug("control subscriber ", sub.id, " lagged behind")
		_ = sub.conn.Close()
	}
}
