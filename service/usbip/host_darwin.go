//go:build darwin && cgo

package usbip

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"slices"
	"sync"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/unix"
)

func newPlatformExportHost(logger log.ContextLogger, matches []option.USBIPDeviceMatch) (ExportHost, error) {
	return newDarwinExportHost(logger, matches), nil
}

func newPlatformImportHost(logger log.ContextLogger) (ImportHost, error) {
	return &darwinImportHost{logger: logger}, nil
}

// darwinExportHost retains stale captures: devices that reconcile
// wanted to drop while still busy stay owned until the import ends.
type darwinExportHost struct {
	logger  log.ContextLogger
	matches []option.USBIPDeviceMatch

	runCtx    context.Context
	runCancel context.CancelFunc

	access  sync.Mutex
	exports map[string]*darwinExport
	watcher *darwinUSBHostDeviceWatcher
}

func newDarwinExportHost(logger log.ContextLogger, matches []option.USBIPDeviceMatch) *darwinExportHost {
	return &darwinExportHost{
		logger:  logger,
		matches: matches,
		exports: make(map[string]*darwinExport),
	}
}

func (h *darwinExportHost) Start(ctx context.Context) error {
	h.runCtx, h.runCancel = context.WithCancel(ctx)
	return nil
}

func (h *darwinExportHost) Close() error {
	if h.runCancel != nil {
		h.runCancel()
	}
	h.access.Lock()
	watcher := h.watcher
	h.watcher = nil
	exports := h.exports
	h.exports = make(map[string]*darwinExport)
	h.access.Unlock()
	if watcher != nil {
		watcher.Close()
	}
	for _, exp := range exports {
		if exp.device != nil {
			exp.device.Close()
		}
	}
	return nil
}

func (h *darwinExportHost) Events() (<-chan struct{}, error) {
	if h.runCtx == nil {
		return nil, E.New("usbip host: Events called before Start")
	}
	ch := make(chan struct{}, 1)
	signal := func() {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	watcher, err := darwinWatchUSBHostDevices(signal)
	if err != nil {
		close(ch)
		return nil, err
	}
	h.access.Lock()
	h.watcher = watcher
	h.access.Unlock()
	go func() {
		<-h.runCtx.Done()
		h.access.Lock()
		w := h.watcher
		h.watcher = nil
		h.access.Unlock()
		if w != nil {
			w.Close()
		}
		close(ch)
	}()
	return ch, nil
}

func (h *darwinExportHost) Reconcile(ctx context.Context, isReserved func(busid string) bool) (map[string]Export, []string, error) {
	devices, err := darwinCopyUSBHostDevices()
	if err != nil {
		return h.snapshotSelf(), nil, E.Cause(err, "enumerate IOUSBHost devices")
	}
	keys := make([]DeviceKey, len(devices))
	for i := range devices {
		keys[i] = devices[i].key
	}
	desired := make(map[string]darwinUSBHostDeviceInfo)
	for _, idx := range SelectMatches(h.matches, keys) {
		if devices[idx].entry.Info.BDeviceClass == 0x09 {
			h.logger.Warn("skip hub device ", devices[idx].key.BusID)
			continue
		}
		desired[devices[idx].key.BusID] = devices[idx]
	}

	h.access.Lock()
	current := make(map[string]*darwinExport, len(h.exports))
	maps.Copy(current, h.exports)
	h.access.Unlock()

	var (
		toAdd    []*darwinExport
		toRemove []*darwinExport
		toStale  []darwinStaleMark
		released []string
	)
	for busid, info := range desired {
		if exp, ok := current[busid]; ok && exp.registryID == info.registryID {
			continue
		}
		if exp, ok := current[busid]; ok {
			if isReserved(busid) {
				toStale = append(toStale, darwinStaleMark{busid: busid, pendingRegistryID: info.registryID})
				continue
			}
			toRemove = append(toRemove, exp)
			released = append(released, busid)
		}
		device, err := darwinOpenUSBHostDevice(info.registryID, true)
		if err != nil {
			h.logger.Warn("capture ", busid, ": ", err)
			continue
		}
		info = device.info
		toAdd = append(toAdd, &darwinExport{
			busid:      info.key.BusID,
			registryID: info.registryID,
			device:     device,
			entry:      info.entry,
			logger:     h.logger,
		})
		h.logger.Info("exported ", info.key.BusID, " through IOUSBHost capture")
	}
	for busid, exp := range current {
		if _, ok := desired[busid]; ok {
			continue
		}
		if isReserved(busid) {
			toStale = append(toStale, darwinStaleMark{busid: busid})
			continue
		}
		toRemove = append(toRemove, exp)
		h.logger.Info("released ", busid, " from IOUSBHost capture")
		released = append(released, busid)
	}

	committed := make(map[string]*darwinExport, len(current)+len(toAdd))
	maps.Copy(committed, current)
	for _, mark := range toStale {
		exp, found := committed[mark.busid]
		if !found {
			continue
		}
		cloned := cloneDarwinExport(exp)
		cloned.stale = true
		if mark.pendingRegistryID != 0 {
			cloned.pendingRegistryID = mark.pendingRegistryID
		}
		committed[mark.busid] = cloned
	}
	for _, exp := range toRemove {
		delete(committed, exp.busid)
	}
	for _, exp := range toAdd {
		committed[exp.busid] = exp
	}

	h.access.Lock()
	h.exports = committed
	h.access.Unlock()

	for _, exp := range toRemove {
		if exp.device != nil {
			exp.device.Close()
		}
	}
	return snapshotDarwinExports(committed), released, nil
}

func (h *darwinExportHost) FinishImport(ctx context.Context, busid string) (bool, error) {
	h.access.Lock()
	exp, ok := h.exports[busid]
	if !ok || !exp.stale {
		h.access.Unlock()
		return false, nil
	}
	pending := exp.pendingRegistryID
	if pending == 0 {
		delete(h.exports, busid)
		h.access.Unlock()
		if exp.device != nil {
			exp.device.Close()
		}
		return true, nil
	}
	h.access.Unlock()

	device, err := darwinOpenUSBHostDevice(pending, true)
	if err != nil {
		h.logger.Warn("re-capture ", busid, " (registry ", pending, "): ", err)
		h.access.Lock()
		delete(h.exports, busid)
		h.access.Unlock()
		if exp.device != nil {
			exp.device.Close()
		}
		return true, nil
	}
	info := device.info
	replacement := &darwinExport{
		busid:      info.key.BusID,
		registryID: info.registryID,
		device:     device,
		entry:      info.entry,
		logger:     h.logger,
	}
	h.access.Lock()
	h.exports[busid] = replacement
	h.access.Unlock()
	if exp.device != nil {
		exp.device.Close()
	}
	h.logger.Info("re-exported ", busid, " through IOUSBHost re-capture (registry ", pending, ")")
	return true, nil
}

func (h *darwinExportHost) snapshotSelf() map[string]Export {
	h.access.Lock()
	defer h.access.Unlock()
	return snapshotDarwinExports(h.exports)
}

// snapshotDarwinExports returns every tracked export, including stale
// ones, matching the ExportSnapshot contract: stale exports surface
// to the ledger so they broadcast as State: unavailable updates
// instead of disappearing.
func snapshotDarwinExports(exports map[string]*darwinExport) map[string]Export {
	out := make(map[string]Export, len(exports))
	for busid, exp := range exports {
		out[busid] = exp
	}
	return out
}

func cloneDarwinExport(exp *darwinExport) *darwinExport {
	if exp == nil {
		return nil
	}
	clone := *exp
	clone.entry.Interfaces = slices.Clone(exp.entry.Interfaces)
	return &clone
}

type darwinExport struct {
	busid             string
	registryID        uint64
	pendingRegistryID uint64
	device            *darwinUSBHostDevice
	entry             DeviceEntry
	logger            log.ContextLogger
	stale             bool
}

type darwinStaleMark struct {
	busid             string
	pendingRegistryID uint64
}

func (e *darwinExport) BusID() string {
	return e.busid
}

func (e *darwinExport) LeaseIdentity() ExportLeaseIdentity {
	return ExportLeaseIdentity(fmt.Sprintf("darwin:%016x", e.registryID))
}

func (e *darwinExport) staleReason() string {
	if e.pendingRegistryID != 0 {
		return "device replaced"
	}
	return "capture released"
}

func (e *darwinExport) Snapshot(ctx context.Context, busy bool) ExportSnapshot {
	stableID := fmt.Sprintf("darwin-registry:%016x", e.registryID)
	if e.stale {
		return ExportSnapshot{
			Entry:        e.entry,
			Backend:      backendIDDarwinIOKit,
			StableID:     stableID,
			State:        deviceStateUnavailable,
			StatusReason: e.staleReason(),
		}
	}
	state := deviceStateAvailable
	if busy {
		state = deviceStateBusy
	}
	return ExportSnapshot{
		Entry:    e.entry,
		Backend:  backendIDDarwinIOKit,
		StableID: stableID,
		State:    state,
	}
}

func (e *darwinExport) LeaseCheck(ctx context.Context) (bool, string) {
	if e.stale {
		return false, e.staleReason()
	}
	return true, ""
}

func (e *darwinExport) DeviceInfo(ctx context.Context) (DeviceInfoTruncated, error) {
	return e.entry.Info, nil
}

func (e *darwinExport) NewServerDataSession(ctx context.Context, conn net.Conn) (DataSession, error) {
	if e.device == nil {
		return nil, E.New("darwin export ", e.busid, " has no device handle")
	}
	return newDarwinServerDataSession(ctx, e.logger, conn, e.device), nil
}

type darwinImportHost struct {
	logger log.ContextLogger
}

func (h *darwinImportHost) Start(ctx context.Context) error {
	return nil
}

func (h *darwinImportHost) Close() error {
	return nil
}

func (h *darwinImportHost) Attach(ctx context.Context, info DeviceInfoTruncated, conn net.Conn) (AttachedSession, error) {
	controller := newDarwinVirtualController(ctx, h.logger, conn, info)
	err := controller.Start()
	if err != nil {
		_ = controller.Close()
		return nil, err
	}
	return controller, nil
}

var _ DataSession = (*darwinServerDataSession)(nil)

type darwinServerDataSession struct {
	ctx         context.Context
	logger      log.ContextLogger
	conn        net.Conn
	device      *darwinUSBHostDevice
	writeAccess sync.Mutex
	access      sync.Mutex
	pending     map[uint32]darwinServerSubmitState
	endpoints   map[uint8]*darwinServerEndpointState
	wg          sync.WaitGroup

	done     chan struct{}
	doneOnce sync.Once
	runErr   error

	stateAccess sync.Mutex
	started     bool
	closed      bool
	closeOnce   sync.Once
	closeErr    error
}

type darwinServerSubmitState struct {
	command  SubmitCommand
	endpoint uint8
	started  bool
	unlinked bool
	drained  chan struct{}
}

type darwinServerEndpointState struct {
	active uint32
	queued []uint32
}

type darwinServerNextSubmit struct {
	sequence uint32
	command  SubmitCommand
}

func newDarwinServerDataSession(ctx context.Context, logger log.ContextLogger, conn net.Conn, device *darwinUSBHostDevice) *darwinServerDataSession {
	return &darwinServerDataSession{
		ctx:       ctx,
		logger:    logger,
		conn:      conn,
		device:    device,
		pending:   make(map[uint32]darwinServerSubmitState),
		endpoints: make(map[uint8]*darwinServerEndpointState),
		done:      make(chan struct{}),
	}
}

func (s *darwinServerDataSession) Done() <-chan struct{} {
	return s.done
}

func (s *darwinServerDataSession) Err() error {
	return s.runErr
}

func (s *darwinServerDataSession) Start() error {
	s.stateAccess.Lock()
	defer s.stateAccess.Unlock()
	if s.started || s.closed {
		return nil
	}
	s.started = true
	go s.run()
	return nil
}

func (s *darwinServerDataSession) Close() error {
	s.closeOnce.Do(func() {
		s.closeErr = common.Close(s.conn)
	})
	s.stateAccess.Lock()
	started := s.started
	s.closed = true
	s.stateAccess.Unlock()
	if started {
		<-s.done
	} else {
		s.markDone(nil)
	}
	return s.closeErr
}

func (s *darwinServerDataSession) markDone(err error) {
	s.doneOnce.Do(func() {
		s.runErr = err
		close(s.done)
	})
}

func (s *darwinServerDataSession) run() {
	err := s.serve()
	if err != nil && (errors.Is(err, io.EOF) || E.IsClosedOrCanceled(err)) {
		err = nil
	}
	s.markDone(err)
}

func (s *darwinServerDataSession) serve() error {
	stopCloseOnCancel := closeConnOnContextDone(s.ctx, s.conn)
	defer stopCloseOnCancel()
	defer func() {
		s.abortPendingSubmits()
		s.wg.Wait()
	}()
	for {
		header, err := ReadDataHeader(s.conn)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		switch header.Command {
		case CmdSubmit:
			command, err := ReadSubmitCommandBody(s.conn, header)
			if err != nil {
				return err
			}
			next, shouldStart := s.enqueueSubmit(command)
			if shouldStart {
				s.startSubmit(next)
			}
		case CmdUnlink:
			command, err := ReadUnlinkCommandBody(s.conn, header)
			if err != nil {
				return err
			}
			status := int32(0)
			endpoint, drained, shouldAbort, found := s.unlinkSubmit(command.SeqNum)
			if found {
				if shouldAbort {
					abortErr := s.device.abortEndpoint(endpoint)
					if abortErr != nil {
						s.logger.Debug("abort endpoint 0x", hex8(endpoint), ": ", abortErr)
					}
				}
				<-drained
				status = usbipStatusECONNRESET
			}
			s.writeAccess.Lock()
			err = WriteUnlinkResponse(s.conn, UnlinkResponse{
				Header: DataHeader{Command: RetUnlink, SeqNum: header.SeqNum, DevID: header.DevID, Direction: header.Direction, Endpoint: header.Endpoint},
				Status: status,
			})
			s.writeAccess.Unlock()
			if err != nil {
				return err
			}
		default:
			return E.New("unexpected USB/IP command ", fmt.Sprintf("0x%08x", header.Command))
		}
	}
}

func (s *darwinServerDataSession) enqueueSubmit(command SubmitCommand) (darwinServerNextSubmit, bool) {
	endpoint := submitScheduleEndpoint(command)
	sequence := command.Header.SeqNum

	s.access.Lock()
	defer s.access.Unlock()

	state := darwinServerSubmitState{
		command:  command,
		endpoint: endpoint,
	}
	endpointState, found := s.endpoints[endpoint]
	if !found {
		endpointState = &darwinServerEndpointState{}
		s.endpoints[endpoint] = endpointState
	}
	if endpointState.active == 0 {
		state.started = true
		s.pending[sequence] = state
		endpointState.active = sequence
		return darwinServerNextSubmit{
			sequence: sequence,
			command:  command,
		}, true
	}
	s.pending[sequence] = state
	endpointState.queued = append(endpointState.queued, sequence)
	return darwinServerNextSubmit{}, false
}

func (s *darwinServerDataSession) startSubmit(next darwinServerNextSubmit) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		response := s.handleSubmit(next.command)
		shouldSend, followUp, hasFollowUp := s.finishSubmit(next.sequence)
		if shouldSend {
			s.writeAccess.Lock()
			err := WriteSubmitResponse(s.conn, response)
			s.writeAccess.Unlock()
			if err != nil {
				_ = s.conn.Close()
			}
		}
		if hasFollowUp {
			s.startSubmit(followUp)
		}
	}()
}

func (s *darwinServerDataSession) handleSubmit(command SubmitCommand) SubmitResponse {
	response := SubmitResponse{
		Header: DataHeader{
			Command:   RetSubmit,
			SeqNum:    command.Header.SeqNum,
			DevID:     command.Header.DevID,
			Direction: command.Header.Direction,
			Endpoint:  command.Header.Endpoint,
		},
		StartFrame:      command.StartFrame,
		NumberOfPackets: command.NumberOfPackets,
		IsoPackets:      slices.Clone(command.IsoPackets),
	}
	buffer := command.Buffer
	if command.Header.Direction == USBIPDirIn && command.TransferBufferLength > 0 {
		buffer = make([]byte, int(command.TransferBufferLength))
	}
	var (
		status int32
		actual int32
		err    error
	)
	endpoint := commandEndpoint(command)
	switch {
	case command.Header.Endpoint == 0:
		status, actual, buffer, err = s.device.control(command.Setup, buffer)
	case command.NumberOfPackets > 0:
		asap := command.TransferFlags&usbipTransferFlagIsoASAP != 0
		status, actual, buffer, response.IsoPackets, err = s.device.iso(endpoint, buffer, command.StartFrame, asap, response.IsoPackets)
	default:
		status, actual, buffer, err = s.device.io(endpoint, buffer)
	}
	if err != nil {
		s.logger.Debug("submit seq ", command.Header.SeqNum, " endpoint 0x", hex8(endpoint), ": ", err)
		response.Status = -int32(unix.EIO)
		return response
	}
	response.Status = status
	if actual < 0 {
		actual = 0
	}
	response.ActualLength = actual
	if command.Header.Direction == USBIPDirIn && actual > 0 {
		if command.NumberOfPackets > 0 {
			response.Buffer = packIsoInResponseBuffer(buffer, response.IsoPackets)
			response.ActualLength = int32(len(response.Buffer))
		} else {
			response.Buffer = buffer[:min(int(actual), len(buffer))]
		}
	}
	return response
}

func packIsoInResponseBuffer(buffer []byte, packets []IsoPacketDescriptor) []byte {
	var total int
	for i := range packets {
		length := int(packets[i].ActualLength)
		if length <= 0 {
			packets[i].ActualLength = 0
			continue
		}
		offset := int(packets[i].Offset)
		if offset < 0 || offset >= len(buffer) {
			packets[i].ActualLength = 0
			continue
		}
		if offset+length > len(buffer) {
			length = len(buffer) - offset
			packets[i].ActualLength = int32(length)
		}
		total += length
	}
	if total == 0 {
		return nil
	}
	packed := make([]byte, 0, total)
	for i := range packets {
		length := int(packets[i].ActualLength)
		if length <= 0 {
			continue
		}
		offset := int(packets[i].Offset)
		packed = append(packed, buffer[offset:offset+length]...)
	}
	return packed
}

func (s *darwinServerDataSession) unlinkSubmit(seq uint32) (uint8, <-chan struct{}, bool, bool) {
	var drained chan struct{}

	s.access.Lock()
	pending, found := s.pending[seq]
	if !found {
		s.access.Unlock()
		return 0, nil, false, false
	}
	if pending.drained == nil {
		pending.drained = make(chan struct{})
	}
	drained = pending.drained
	if !pending.started {
		endpointState := s.endpoints[pending.endpoint]
		if endpointState != nil {
			endpointState.queued = removeQueuedSequence(endpointState.queued, seq)
			if endpointState.active == 0 && len(endpointState.queued) == 0 {
				delete(s.endpoints, pending.endpoint)
			}
		}
		delete(s.pending, seq)
		s.access.Unlock()
		close(drained)
		return pending.endpoint, drained, false, true
	}
	shouldAbort := !pending.unlinked
	pending.unlinked = true
	s.pending[seq] = pending
	s.access.Unlock()
	return pending.endpoint, drained, shouldAbort, true
}

func (s *darwinServerDataSession) finishSubmit(seq uint32) (bool, darwinServerNextSubmit, bool) {
	var drained chan struct{}
	var followUp darwinServerNextSubmit
	var hasFollowUp bool

	s.access.Lock()
	pending, found := s.pending[seq]
	if !found {
		s.access.Unlock()
		return true, darwinServerNextSubmit{}, false
	}
	endpointState := s.endpoints[pending.endpoint]
	if endpointState != nil && endpointState.active == seq {
		endpointState.active = 0
	}
	delete(s.pending, seq)
	if endpointState != nil {
		for len(endpointState.queued) > 0 {
			nextSequence := endpointState.queued[0]
			endpointState.queued = endpointState.queued[1:]
			nextPending, nextFound := s.pending[nextSequence]
			if !nextFound {
				continue
			}
			nextPending.started = true
			s.pending[nextSequence] = nextPending
			endpointState.active = nextSequence
			followUp = darwinServerNextSubmit{
				sequence: nextSequence,
				command:  nextPending.command,
			}
			hasFollowUp = true
			break
		}
		if endpointState.active == 0 && len(endpointState.queued) == 0 {
			delete(s.endpoints, pending.endpoint)
		}
	}
	drained = pending.drained
	unlinked := pending.unlinked
	s.access.Unlock()
	if drained != nil {
		close(drained)
	}
	return !unlinked, followUp, hasFollowUp
}

func (s *darwinServerDataSession) abortPendingSubmits() {
	var (
		activeEndpoints []uint8
		drained         []chan struct{}
	)

	s.access.Lock()
	seen := make(map[uint8]struct{})
	for seq, pending := range s.pending {
		if !pending.started {
			delete(s.pending, seq)
			if pending.drained != nil {
				drained = append(drained, pending.drained)
			}
			continue
		}
		if !pending.unlinked {
			seen[pending.endpoint] = struct{}{}
		}
		pending.unlinked = true
		s.pending[seq] = pending
	}
	for endpoint := range s.endpoints {
		endpointState := s.endpoints[endpoint]
		if endpointState != nil {
			endpointState.queued = nil
		}
	}
	s.access.Unlock()

	for _, drainedChannel := range drained {
		close(drainedChannel)
	}
	activeEndpoints = make([]uint8, 0, len(seen))
	for endpoint := range seen {
		activeEndpoints = append(activeEndpoints, endpoint)
	}
	slices.Sort(activeEndpoints)
	if s.device == nil {
		return
	}
	for _, endpoint := range activeEndpoints {
		err := s.device.abortEndpoint(endpoint)
		if err != nil {
			s.logger.Debug("abort endpoint 0x", hex8(endpoint), ": ", err)
		}
	}
}

func removeQueuedSequence(queue []uint32, sequence uint32) []uint32 {
	for index, current := range queue {
		if current != sequence {
			continue
		}
		return append(queue[:index], queue[index+1:]...)
	}
	return queue
}

func submitScheduleEndpoint(command SubmitCommand) uint8 {
	if command.Header.Endpoint == 0 {
		return 0
	}
	return commandEndpoint(command)
}

func commandEndpoint(command SubmitCommand) uint8 {
	endpoint := uint8(command.Header.Endpoint & 0x0f)
	if command.Header.Direction == USBIPDirIn {
		endpoint |= 0x80
	}
	return endpoint
}
