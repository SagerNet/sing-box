//go:build darwin && cgo

package usbip

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	return newDarwinImportHost(logger), nil
}

// darwinExportHost retains stale captures: devices that reconcile
// wanted to drop while still busy stay owned until the import ends.
type darwinExportHost struct {
	logger  log.ContextLogger
	matches []option.USBIPDeviceMatch

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
	return nil
}

func (h *darwinExportHost) Close() error {
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

func (h *darwinExportHost) Events(ctx context.Context) (<-chan struct{}, error) {
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
		<-ctx.Done()
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

func (h *darwinExportHost) Reconcile(ctx context.Context, isBusy func(busid string) bool) (map[string]Export, []string, error) {
	devices, err := darwinCopyUSBHostDevices()
	if err != nil {
		return h.snapshotSelf(), nil, E.Cause(err, "enumerate IOUSBHost devices")
	}
	desired := make(map[string]darwinUSBHostDeviceInfo)
	for _, match := range h.matches {
		for i := range devices {
			if !matches(match, devices[i].key) {
				continue
			}
			if devices[i].entry.Info.BDeviceClass == 0x09 {
				h.logger.Warn("skip hub device ", devices[i].key.BusID, " matched by ", describeMatch(match))
				continue
			}
			desired[devices[i].key.BusID] = devices[i]
		}
	}

	h.access.Lock()
	current := make(map[string]*darwinExport, len(h.exports))
	for busid, exp := range h.exports {
		current[busid] = exp
	}
	h.access.Unlock()

	var (
		toAdd    []*darwinExport
		toRemove []*darwinExport
		toStale  []string
		released []string
	)
	for busid, info := range desired {
		if exp, ok := current[busid]; ok && exp.registryID == info.registryID {
			continue
		}
		if exp, ok := current[busid]; ok {
			if isBusy(busid) {
				toStale = append(toStale, busid)
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
		if isBusy(busid) {
			toStale = append(toStale, busid)
			continue
		}
		toRemove = append(toRemove, exp)
		h.logger.Info("released ", busid, " from IOUSBHost capture")
		released = append(released, busid)
	}

	h.access.Lock()
	for _, busid := range toStale {
		exp, ok := h.exports[busid]
		if !ok || exp.stale {
			continue
		}
		exp.stale = true
	}
	for _, exp := range toRemove {
		delete(h.exports, exp.busid)
	}
	for _, exp := range toAdd {
		h.exports[exp.busid] = exp
	}
	out := make(map[string]Export, len(h.exports))
	for busid, exp := range h.exports {
		if exp.stale {
			continue
		}
		out[busid] = exp
	}
	h.access.Unlock()

	for _, exp := range toRemove {
		if exp.device != nil {
			exp.device.Close()
		}
	}
	return out, released, nil
}

func (h *darwinExportHost) FinishImport(ctx context.Context, busid string) (bool, error) {
	h.access.Lock()
	exp, ok := h.exports[busid]
	if !ok || !exp.stale {
		h.access.Unlock()
		return false, nil
	}
	delete(h.exports, busid)
	h.access.Unlock()
	if exp.device != nil {
		exp.device.Close()
	}
	return true, nil
}

func (h *darwinExportHost) snapshotSelf() map[string]Export {
	h.access.Lock()
	defer h.access.Unlock()
	out := make(map[string]Export, len(h.exports))
	for busid, exp := range h.exports {
		if exp.stale {
			continue
		}
		out[busid] = exp
	}
	return out
}

type darwinExport struct {
	busid      string
	registryID uint64
	device     *darwinUSBHostDevice
	entry      DeviceEntry
	logger     log.ContextLogger
	stale      bool
}

func (e *darwinExport) BusID() string {
	return e.busid
}

func (e *darwinExport) Snapshot(ctx context.Context, busy bool) ExportSnapshot {
	stableID := fmt.Sprintf("darwin-registry:%016x", e.registryID)
	if e.stale {
		return ExportSnapshot{
			Backend:  backendIDDarwinIOKit,
			StableID: stableID,
			State:    deviceStateUnavailable,
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
		return false, "capture released"
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

func newDarwinImportHost(logger log.ContextLogger) *darwinImportHost {
	return &darwinImportHost{logger: logger}
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
	return &darwinClientSession{controller: controller}, nil
}

type darwinClientSession struct {
	controller *darwinVirtualController
}

func (s *darwinClientSession) Done() <-chan struct{} {
	return s.controller.Done()
}

func (s *darwinClientSession) Err() error {
	return s.controller.Err()
}

func (s *darwinClientSession) Close() error {
	return s.controller.Close()
}

func (s *darwinClientSession) Description() string {
	return "IOUSBHostControllerInterface"
}

var _ DataSession = (*darwinServerDataSession)(nil)

type darwinServerDataSession struct {
	ctx         context.Context
	logger      log.ContextLogger
	conn        net.Conn
	device      *darwinUSBHostDevice
	writeAccess sync.Mutex
	access      sync.Mutex
	pending     map[uint32]darwinServerPendingSubmit
	wg          sync.WaitGroup

	done      chan struct{}
	doneOnce  sync.Once
	runErr    error
	closeOnce sync.Once
	closeErr  error
}

type darwinServerPendingSubmit struct {
	endpoint uint8
	unlinked bool
}

func newDarwinServerDataSession(ctx context.Context, logger log.ContextLogger, conn net.Conn, device *darwinUSBHostDevice) *darwinServerDataSession {
	session := &darwinServerDataSession{
		ctx:     ctx,
		logger:  logger,
		conn:    conn,
		device:  device,
		pending: make(map[uint32]darwinServerPendingSubmit),
		done:    make(chan struct{}),
	}
	go session.run()
	return session
}

func (s *darwinServerDataSession) Done() <-chan struct{} {
	return s.done
}

func (s *darwinServerDataSession) Err() error {
	return s.runErr
}

func (s *darwinServerDataSession) Close() error {
	s.closeOnce.Do(func() {
		s.closeErr = common.Close(s.conn)
	})
	<-s.done
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
			s.trackSubmit(command.Header.SeqNum, commandEndpoint(command))
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				response := s.handleSubmit(command)
				if !s.finishSubmit(command.Header.SeqNum) {
					return
				}
				s.writeAccess.Lock()
				err := WriteSubmitResponse(s.conn, response)
				s.writeAccess.Unlock()
				if err != nil {
					_ = s.conn.Close()
				}
			}()
		case CmdUnlink:
			command, err := ReadUnlinkCommandBody(s.conn, header)
			if err != nil {
				return err
			}
			status := int32(0)
			if endpoint, ok := s.markSubmitUnlinked(command.SeqNum); ok {
				abortErr := s.device.abortEndpoint(endpoint)
				if abortErr != nil {
					s.logger.Debug("abort endpoint 0x", hex8(endpoint), ": ", abortErr)
				}
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
			return E.New(fmt.Sprintf("unexpected USB/IP command 0x%08x", header.Command))
		}
	}
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
		status, actual, buffer, response.IsoPackets, err = s.device.iso(endpoint, buffer, command.StartFrame, response.IsoPackets)
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
		response.Buffer = buffer[:min(int(actual), len(buffer))]
	}
	return response
}

func (s *darwinServerDataSession) trackSubmit(seq uint32, endpoint uint8) {
	s.access.Lock()
	defer s.access.Unlock()
	s.pending[seq] = darwinServerPendingSubmit{endpoint: endpoint}
}

func (s *darwinServerDataSession) markSubmitUnlinked(seq uint32) (uint8, bool) {
	s.access.Lock()
	defer s.access.Unlock()
	pending, ok := s.pending[seq]
	if !ok {
		return 0, false
	}
	pending.unlinked = true
	s.pending[seq] = pending
	return pending.endpoint, true
}

func (s *darwinServerDataSession) finishSubmit(seq uint32) bool {
	s.access.Lock()
	defer s.access.Unlock()
	pending, ok := s.pending[seq]
	if !ok {
		return true
	}
	delete(s.pending, seq)
	return !pending.unlinked
}

func (s *darwinServerDataSession) abortPendingSubmits() {
	s.access.Lock()
	seen := make(map[uint8]struct{})
	for seq, pending := range s.pending {
		if !pending.unlinked {
			seen[pending.endpoint] = struct{}{}
		}
		pending.unlinked = true
		s.pending[seq] = pending
	}
	endpoints := make([]uint8, 0, len(seen))
	for endpoint := range seen {
		endpoints = append(endpoints, endpoint)
	}
	slices.Sort(endpoints)
	s.access.Unlock()
	if s.device == nil {
		return
	}
	for _, endpoint := range endpoints {
		err := s.device.abortEndpoint(endpoint)
		if err != nil {
			s.logger.Debug("abort endpoint 0x", hex8(endpoint), ": ", err)
		}
	}
}

func commandEndpoint(command SubmitCommand) uint8 {
	endpoint := uint8(command.Header.Endpoint & 0x0f)
	if command.Header.Direction == USBIPDirIn {
		endpoint |= 0x80
	}
	return endpoint
}
