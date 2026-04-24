//go:build darwin && cgo

package usbip

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"strings"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	boxService "github.com/sagernet/sing-box/adapter/service"
	"github.com/sagernet/sing-box/common/listener"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"

	"golang.org/x/sys/unix"
)

type serverExport struct {
	busid      string
	registryID uint64
	device     *darwinUSBHostDevice
	entry      DeviceEntry
	busy       bool
}

type darwinUSBHostDeviceWatch interface {
	Close()
}

type darwinServerOps struct {
	copyUSBHostDevices  func() ([]darwinUSBHostDeviceInfo, error)
	openUSBHostDevice   func(registryID uint64, capture bool) (*darwinUSBHostDevice, error)
	watchUSBHostDevices func(func()) (darwinUSBHostDeviceWatch, error)
}

var systemDarwinServerOps = darwinServerOps{
	copyUSBHostDevices:  darwinCopyUSBHostDevices,
	openUSBHostDevice:   darwinOpenUSBHostDevice,
	watchUSBHostDevices: darwinWatchUSBHostDevices,
}

type ServerService struct {
	boxService.Adapter
	ctx      context.Context
	cancel   context.CancelFunc
	logger   log.ContextLogger
	listener *listener.Listener
	matches  []option.USBIPDeviceMatch
	ops      darwinServerOps

	access  sync.Mutex
	exports map[string]serverExport
	listen  net.Listener
	watcher darwinUSBHostDeviceWatch

	controlAccess sync.Mutex
	controlSeq    uint64
	controlNextID uint64
	controlSubs   map[uint64]*serverControlConn
	controlState  map[string]DeviceInfoV2
	leaseNextID   uint64
	leasesByBusID map[string]serverImportLease

	reconcileAccess sync.Mutex
}

func NewServerService(ctx context.Context, logger log.ContextLogger, tag string, options option.USBIPServerServiceOptions) (adapter.Service, error) {
	for i, m := range options.Devices {
		if m.IsZero() {
			return nil, E.New("devices[", i, "]: at least one of busid/vendor_id/product_id/serial is required")
		}
	}
	if options.ListenPort == 0 {
		options.ListenPort = DefaultPort
	}
	ctx, cancel := context.WithCancel(ctx)
	return &ServerService{
		Adapter: boxService.NewAdapter(C.TypeUSBIPServer, tag),
		ctx:     ctx,
		cancel:  cancel,
		logger:  logger,
		matches: options.Devices,
		exports: make(map[string]serverExport),
		listener: listener.New(listener.Options{
			Context: ctx,
			Logger:  logger,
			Network: []string{N.NetworkTCP},
			Listen:  options.ListenOptions,
		}),
		controlSubs:   make(map[uint64]*serverControlConn),
		controlState:  make(map[string]DeviceInfoV2),
		leasesByBusID: make(map[string]serverImportLease),
		ops:           systemDarwinServerOps,
	}, nil
}

func (s *ServerService) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	err := s.reconcileAndBroadcast(false)
	if err != nil {
		return err
	}
	watcher, err := s.newUSBEventWatcher()
	if err != nil {
		s.rollbackExports()
		return err
	}
	var tcpListener net.Listener
	tcpListener, err = s.listener.ListenTCP()
	if err != nil {
		watcher.Close()
		s.rollbackExports()
		return err
	}
	s.access.Lock()
	s.listen = tcpListener
	s.watcher = watcher
	s.access.Unlock()
	go s.acceptLoop(tcpListener)
	return nil
}

func (s *ServerService) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
	s.closeControlSubscribers()
	err := common.Close(common.PtrOrNil(s.listener))
	s.access.Lock()
	watcher := s.watcher
	s.watcher = nil
	s.access.Unlock()
	if watcher != nil {
		watcher.Close()
	}
	s.reconcileAccess.Lock()
	defer s.reconcileAccess.Unlock()
	s.rollbackExports()
	return err
}

func (s *ServerService) newUSBEventWatcher() (darwinUSBHostDeviceWatch, error) {
	ops := s.darwinOps()
	return ops.watchUSBHostDevices(func() {
		err := s.reconcileAndBroadcast(true)
		if err != nil {
			s.logger.Warn("reconcile exports: ", err)
		}
	})
}

func (s *ServerService) darwinOps() darwinServerOps {
	ops := s.ops
	if ops.copyUSBHostDevices == nil {
		ops.copyUSBHostDevices = darwinCopyUSBHostDevices
	}
	if ops.openUSBHostDevice == nil {
		ops.openUSBHostDevice = darwinOpenUSBHostDevice
	}
	if ops.watchUSBHostDevices == nil {
		ops.watchUSBHostDevices = darwinWatchUSBHostDevices
	}
	return ops
}

func (s *ServerService) reconcileExports() (bool, error) {
	ops := s.darwinOps()
	devices, err := ops.copyUSBHostDevices()
	if err != nil {
		return false, E.Cause(err, "enumerate IOUSBHost devices")
	}
	desired := make(map[string]darwinUSBHostDeviceInfo)
	for _, match := range s.matches {
		for i := range devices {
			if !matches(match, devices[i].key) {
				continue
			}
			if devices[i].entry.Info.BDeviceClass == 0x09 {
				s.logger.Warn("skip hub device ", devices[i].key.BusID, " matched by ", describeMatch(match))
				continue
			}
			desired[devices[i].key.BusID] = devices[i]
		}
	}

	current := s.snapshotExports()
	changed := false
	for busid, info := range desired {
		if export, ok := current[busid]; ok && export.registryID == info.registryID {
			continue
		}
		if export, ok := current[busid]; ok {
			if export.busy {
				continue
			}
			s.deleteExport(busid)
			export.device.Close()
			changed = true
		}
		device, err := ops.openUSBHostDevice(info.registryID, true)
		if err != nil {
			s.logger.Warn("capture ", busid, ": ", err)
			continue
		}
		info = device.info
		s.setExport(serverExport{
			busid:      info.key.BusID,
			registryID: info.registryID,
			device:     device,
			entry:      info.entry,
		})
		s.logger.Info("exported ", info.key.BusID, " through IOUSBHost capture")
		changed = true
	}
	for busid, export := range current {
		if _, ok := desired[busid]; ok {
			continue
		}
		if export.busy {
			continue
		}
		s.deleteExport(busid)
		export.device.Close()
		s.logger.Info("released ", busid, " from IOUSBHost capture")
		changed = true
	}
	return changed, nil
}

func (s *ServerService) reconcileAndBroadcast(notify bool) error {
	s.reconcileAccess.Lock()
	defer s.reconcileAccess.Unlock()
	if s.ctx != nil && s.ctx.Err() != nil {
		return nil
	}
	changed, err := s.reconcileExports()
	if err != nil {
		return err
	}
	if notify && changed {
		s.broadcastChanged()
	} else {
		s.refreshControlState()
	}
	return nil
}

func (s *ServerService) rollbackExports() {
	exports := s.snapshotExports()
	for busid, export := range exports {
		s.deleteExport(busid)
		export.device.Close()
	}
}

func (s *ServerService) currentExports() []serverExport {
	s.access.Lock()
	defer s.access.Unlock()
	out := make([]serverExport, 0, len(s.exports))
	for _, export := range s.exports {
		if export.busy {
			continue
		}
		out = append(out, export)
	}
	slices.SortFunc(out, func(a, b serverExport) int {
		return strings.Compare(a.busid, b.busid)
	})
	return out
}

func (s *ServerService) allExports() []serverExport {
	s.access.Lock()
	defer s.access.Unlock()
	out := make([]serverExport, 0, len(s.exports))
	for _, export := range s.exports {
		out = append(out, export)
	}
	slices.SortFunc(out, func(a, b serverExport) int {
		return strings.Compare(a.busid, b.busid)
	})
	return out
}

func (s *ServerService) snapshotExports() map[string]serverExport {
	s.access.Lock()
	defer s.access.Unlock()
	out := make(map[string]serverExport, len(s.exports))
	for busid, export := range s.exports {
		out[busid] = export
	}
	return out
}

func (s *ServerService) setExport(export serverExport) {
	s.access.Lock()
	defer s.access.Unlock()
	s.exports[export.busid] = export
}

func (s *ServerService) deleteExport(busid string) {
	s.access.Lock()
	defer s.access.Unlock()
	delete(s.exports, busid)
}

func (s *ServerService) claimExport(busid string) (serverExport, bool) {
	s.access.Lock()
	defer s.access.Unlock()
	export, ok := s.exports[busid]
	if !ok || export.busy {
		return serverExport{}, false
	}
	export.busy = true
	s.exports[busid] = export
	return export, true
}

func (s *ServerService) releaseClaim(busid string) {
	s.access.Lock()
	export, ok := s.exports[busid]
	if ok {
		export.busy = false
		s.exports[busid] = export
	}
	s.access.Unlock()
}

func (s *ServerService) handleStandardConn(conn net.Conn, header OpHeader) {
	defer conn.Close()
	switch header.Code {
	case OpReqDevList:
		s.handleDevList(conn)
	case OpReqImport:
		s.handleImport(conn)
	case OpReqImportExt:
		s.handleImportExt(conn)
	default:
		s.logger.Debug(fmt.Sprintf("unknown opcode 0x%04x", header.Code))
	}
}

func (s *ServerService) handleControlConn(conn net.Conn) {
	defer conn.Close()
	helloMessage, err := readControlMessage(conn)
	if err != nil {
		s.logger.Debug("read control hello: ", err)
		return
	}
	hello := helloMessage.Frame
	if hello.Type != controlFrameHello || hello.Version != controlProtocolVersion || hello.Capabilities&controlRequiredCapabilities != controlRequiredCapabilities {
		s.logger.Debug("invalid control hello")
		return
	}
	capabilities := negotiatedControlCapabilities(hello.Capabilities)
	sub, seq := s.registerControlConn(conn, capabilities)
	defer s.unregisterControlConn(sub.id)
	err = writeControlAckWithCapabilities(conn, seq, capabilities)
	if err != nil {
		s.logger.Debug("write control ack: ", err)
		return
	}
	readDone := make(chan struct{})
	go s.readControlConn(sub, readDone)
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-readDone:
			return
		case message := <-sub.send:
			err = writeControlMessage(conn, message.Frame, message.Payload)
			if err != nil {
				s.logger.Debug("write control frame: ", err)
				return
			}
		}
	}
}

func (s *ServerService) handleDevList(conn net.Conn) {
	exports := s.currentExports()
	entries := make([]DeviceEntry, 0, len(exports))
	for _, export := range exports {
		entries = append(entries, export.entry)
	}
	err := WriteOpRepDevList(conn, entries)
	if err != nil {
		s.logger.Debug("write devlist: ", err)
	}
}

func (s *ServerService) handleImport(conn net.Conn) {
	busid, err := ReadOpReqImportBody(conn)
	if err != nil {
		s.logger.Debug("read import body: ", err)
		return
	}
	s.handleImportBusID(conn, busid, false)
}

func (s *ServerService) handleImportExt(conn net.Conn) {
	request, err := ReadOpReqImportExtBody(conn)
	if err != nil {
		s.logger.Debug("read import-ext body: ", err)
		return
	}
	if !s.consumeImportLease(request) {
		s.logger.Info("import-ext rejected (invalid lease): ", request.BusID)
		_ = WriteOpRepImportExt(conn, OpStatusError, nil)
		return
	}
	s.handleImportBusID(conn, request.BusID, true)
}

func (s *ServerService) handleImportBusID(conn net.Conn, busid string, extended bool) {
	writeReply := WriteOpRepImport
	if extended {
		writeReply = WriteOpRepImportExt
	}
	export, ok := s.claimExport(busid)
	if !ok {
		s.logger.Info("import rejected (unknown or busy busid): ", busid)
		_ = writeReply(conn, OpStatusError, nil)
		return
	}
	s.broadcastChanged()
	releaseClaim := true
	defer func() {
		if releaseClaim {
			s.releaseClaim(busid)
			s.broadcastChanged()
		}
	}()
	info := export.entry.Info
	err := writeReply(conn, OpStatusOK, &info)
	if err != nil {
		s.logger.Warn("reply import ", busid, ": ", err)
		return
	}
	s.logger.Info("attached ", busid, " to remote ", conn.RemoteAddr())
	session := newDarwinServerDataSession(s.ctx, s.logger, conn, export.device)
	err = session.serve()
	if err != nil && s.ctx.Err() == nil {
		s.logger.Debug("data session ", busid, ": ", err)
	}
	s.releaseClaim(busid)
	releaseClaim = false
	s.broadcastChanged()
}

func (s *ServerService) broadcastChanged() {
	s.broadcastControlState(deviceInfoV2Map(s.buildDeviceStateV2()), false)
}

func (s *ServerService) buildDeviceStateV2() []DeviceInfoV2 {
	exports := s.allExports()
	if len(exports) == 0 {
		return nil
	}
	devices := make([]DeviceInfoV2, 0, len(exports))
	for _, export := range exports {
		state := deviceStateAvailable
		reason := deviceStateAvailable
		if export.busy {
			state = deviceStateBusy
			reason = deviceStateBusy
		}
		devices = append(devices, deviceInfoV2FromEntry(export.entry, backendIDDarwinIOKit, darwinStableID(export.registryID), state, 0, reason))
	}
	return devices
}

func (s *ServerService) leaseAvailable(busid string) (bool, string) {
	s.access.Lock()
	defer s.access.Unlock()
	export, ok := s.exports[busid]
	if !ok {
		return false, "unknown busid"
	}
	if export.busy {
		return false, deviceStateBusy
	}
	return true, ""
}

func darwinStableID(registryID uint64) string {
	return fmt.Sprintf("darwin-registry:%016x", registryID)
}

type darwinServerDataSession struct {
	ctx         context.Context
	logger      log.ContextLogger
	conn        net.Conn
	device      darwinServerDataDevice
	writeAccess sync.Mutex
	access      sync.Mutex
	pending     map[uint32]darwinServerPendingSubmit
	wg          sync.WaitGroup
}

type darwinServerPendingSubmit struct {
	endpoint uint8
	unlinked bool
}

type darwinServerDataDevice interface {
	control(setup [8]byte, buffer []byte) (int32, int32, []byte, error)
	io(endpoint uint8, buffer []byte) (int32, int32, []byte, error)
	iso(endpoint uint8, buffer []byte, startFrame int32, packets []IsoPacketDescriptor) (int32, int32, []byte, []IsoPacketDescriptor, error)
	abortEndpoint(endpoint uint8) error
}

func newDarwinServerDataSession(ctx context.Context, logger log.ContextLogger, conn net.Conn, device darwinServerDataDevice) *darwinServerDataSession {
	return &darwinServerDataSession{
		ctx:     ctx,
		logger:  logger,
		conn:    conn,
		device:  device,
		pending: make(map[uint32]darwinServerPendingSubmit),
	}
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
		IsoPackets:      cloneIsoPackets(command.IsoPackets),
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
	if command.Header.Endpoint == 0 {
		status, actual, buffer, err = s.device.control(command.Setup, buffer)
	} else if command.NumberOfPackets > 0 {
		status, actual, buffer, response.IsoPackets, err = s.device.iso(endpoint, buffer, command.StartFrame, response.IsoPackets)
	} else {
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
	endpoints := s.markPendingSubmitsUnlinked()
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

func (s *darwinServerDataSession) markPendingSubmitsUnlinked() []uint8 {
	s.access.Lock()
	defer s.access.Unlock()
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
	return endpoints
}

func commandEndpoint(command SubmitCommand) uint8 {
	endpoint := uint8(command.Header.Endpoint & 0x0f)
	if command.Header.Direction == USBIPDirIn {
		endpoint |= 0x80
	}
	return endpoint
}

func cloneIsoPackets(in []IsoPacketDescriptor) []IsoPacketDescriptor {
	if len(in) == 0 {
		return nil
	}
	out := make([]IsoPacketDescriptor, len(in))
	copy(out, in)
	return out
}
