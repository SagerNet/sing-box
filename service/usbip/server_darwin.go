//go:build darwin && cgo

package usbip

import (
	"context"
	"errors"
	"io"
	"net"
	"slices"
	"sync"
	"time"

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

type serverControlConn struct {
	id           uint64
	capabilities uint32
	conn         net.Conn
	send         chan controlOutboundMessage
}

type ServerService struct {
	boxService.Adapter
	ctx      context.Context
	cancel   context.CancelFunc
	logger   log.ContextLogger
	listener *listener.Listener
	matches  []option.USBIPDeviceMatch

	mu      sync.Mutex
	exports map[string]serverExport
	listen  net.Listener

	controlMu     sync.Mutex
	controlSeq    uint64
	controlNextID uint64
	controlSubs   map[uint64]*serverControlConn
	controlState  map[string]DeviceInfoV2
	leaseNextID   uint64
	leases        map[uint64]serverImportLease
	leaseByBusID  map[string]uint64

	reconcileMu sync.Mutex
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
		controlSubs:  make(map[uint64]*serverControlConn),
		controlState: make(map[string]DeviceInfoV2),
		leases:       make(map[uint64]serverImportLease),
		leaseByBusID: make(map[string]uint64),
	}, nil
}

func (s *ServerService) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	if err := s.reconcileAndBroadcast(false); err != nil {
		return err
	}
	tcpListener, err := s.listener.ListenTCP()
	if err != nil {
		s.rollbackExports()
		return err
	}
	s.mu.Lock()
	s.listen = tcpListener
	s.mu.Unlock()
	go s.acceptLoop(tcpListener)
	go s.reconcileLoop()
	return nil
}

func (s *ServerService) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
	s.closeControlSubscribers()
	err := common.Close(common.PtrOrNil(s.listener))
	s.rollbackExports()
	return err
}

func (s *ServerService) reconcileExports() (bool, error) {
	devices, err := darwinCopyUSBHostDevices()
	if err != nil {
		return false, E.Cause(err, "enumerate IOUSBHost devices")
	}
	desired := make(map[string]darwinUSBHostDeviceInfo)
	for _, match := range s.matches {
		for i := range devices {
			if !Matches(match, devices[i].key) {
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
		device, err := darwinOpenUSBHostDevice(info.registryID, true)
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
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
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
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]serverExport, 0, len(s.exports))
	for _, export := range s.exports {
		if export.busy {
			continue
		}
		out = append(out, export)
	}
	slices.SortFunc(out, func(a, b serverExport) int {
		return stringsCompare(a.busid, b.busid)
	})
	return out
}

func (s *ServerService) snapshotExports() map[string]serverExport {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]serverExport, len(s.exports))
	for busid, export := range s.exports {
		out[busid] = export
	}
	return out
}

func (s *ServerService) setExport(export serverExport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.exports[export.busid] = export
}

func (s *ServerService) deleteExport(busid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.exports, busid)
}

func (s *ServerService) claimExport(busid string) (serverExport, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	export, ok := s.exports[busid]
	if !ok || export.busy {
		return serverExport{}, false
	}
	export.busy = true
	s.exports[busid] = export
	return export, true
}

func (s *ServerService) releaseClaim(busid string) {
	s.mu.Lock()
	export, ok := s.exports[busid]
	if ok {
		export.busy = false
		s.exports[busid] = export
	}
	s.mu.Unlock()
}

func (s *ServerService) acceptLoop(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
			}
			if E.IsClosed(err) {
				return
			}
			//nolint:staticcheck // Keep parity with Linux accept retry handling.
			if netError, isNetError := err.(net.Error); isNetError && netError.Temporary() {
				s.logger.Error("accept: ", err)
				if !sleepCtx(s.ctx, 200*time.Millisecond) {
					return
				}
				continue
			}
			s.logger.Error("accept: ", err)
			return
		}
		go s.dispatchConn(conn)
	}
}

func (s *ServerService) dispatchConn(conn net.Conn) {
	var prefix [controlPrefaceSize]byte
	if _, err := io.ReadFull(conn, prefix[:]); err != nil {
		s.logger.Debug("read connection preface: ", err)
		_ = conn.Close()
		return
	}
	if IsControlPreface(prefix[:]) {
		s.handleControlConn(conn)
		return
	}
	s.handleStandardConn(conn, ParseOpHeader(prefix[:]))
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
		s.logger.Debug("unknown opcode 0x", hex16(header.Code))
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
	if err := writeControlAckWithCapabilities(conn, seq, capabilities); err != nil {
		s.logger.Debug("write control ack: ", err)
		return
	}
	if supportsControlExtensions(capabilities) {
		s.enqueueControlSnapshot(sub, seq)
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
			if err := writeControlMessage(conn, message.Frame, message.Payload); err != nil {
				s.logger.Debug("write control frame: ", err)
				return
			}
		}
	}
}

func (s *ServerService) readControlConn(sub *serverControlConn, done chan<- struct{}) {
	defer close(done)
	for {
		message, err := readControlMessage(sub.conn)
		if err != nil {
			return
		}
		frame := message.Frame
		switch frame.Type {
		case controlFramePing:
			s.enqueueControlFrame(sub, controlFrame{Type: controlFramePong, Version: controlProtocolVersion})
		case controlFrameLeaseRequest:
			if supportsControlExtensions(sub.capabilities) {
				s.handleControlLeaseRequest(sub, message.Payload)
				continue
			}
			return
		default:
			return
		}
	}
}

func (s *ServerService) handleDevList(conn net.Conn) {
	exports := s.currentExports()
	entries := make([]DeviceEntry, 0, len(exports))
	for _, export := range exports {
		entries = append(entries, export.entry)
	}
	if err := WriteOpRepDevList(conn, entries); err != nil {
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
	releaseClaim := true
	defer func() {
		if releaseClaim {
			s.releaseClaim(busid)
		}
	}()
	info := export.entry.Info
	if err := writeReply(conn, OpStatusOK, &info); err != nil {
		s.logger.Warn("reply import ", busid, ": ", err)
		return
	}
	s.logger.Info("attached ", busid, " to remote ", conn.RemoteAddr())
	session := newDarwinServerDataSession(s.ctx, s.logger, conn, export.device)
	if err := session.serve(); err != nil && s.ctx.Err() == nil {
		s.logger.Debug("data session ", busid, ": ", err)
	}
	s.releaseClaim(busid)
	releaseClaim = false
	s.broadcastChanged()
}

func (s *ServerService) reconcileLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
		}
		if err := s.reconcileAndBroadcast(true); err != nil {
			s.logger.Warn("reconcile exports: ", err)
		}
	}
}

func (s *ServerService) registerControlConn(conn net.Conn, capabilities uint32) (*serverControlConn, uint64) {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	s.controlNextID++
	sub := &serverControlConn{
		id:           s.controlNextID,
		capabilities: capabilities,
		conn:         conn,
		send:         make(chan controlOutboundMessage, 16),
	}
	s.controlSubs[sub.id] = sub
	return sub, s.controlSeq
}

func (s *ServerService) unregisterControlConn(id uint64) {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	delete(s.controlSubs, id)
	s.deleteImportLeasesForSubscriberLocked(id)
}

func (s *ServerService) closeControlSubscribers() {
	s.controlMu.Lock()
	subs := make([]*serverControlConn, 0, len(s.controlSubs))
	for _, sub := range s.controlSubs {
		subs = append(subs, sub)
	}
	s.controlSubs = make(map[uint64]*serverControlConn)
	s.controlMu.Unlock()
	for _, sub := range subs {
		_ = sub.conn.Close()
	}
}

func (s *ServerService) broadcastChanged() {
	devices := s.buildDeviceStateV2()
	nextState := deviceInfoV2Map(devices)

	s.controlMu.Lock()
	s.controlSeq++
	sequence := s.controlSeq
	delta := buildControlDeviceDelta(sequence, s.controlState, nextState)
	s.controlState = nextState
	subs := make([]*serverControlConn, 0, len(s.controlSubs))
	for _, sub := range s.controlSubs {
		subs = append(subs, sub)
	}
	s.controlMu.Unlock()
	frame := controlFrame{Type: controlFrameChanged, Version: controlProtocolVersion, Sequence: sequence}
	for _, sub := range subs {
		if supportsControlExtensions(sub.capabilities) {
			s.enqueueControlPayload(sub, controlFrame{
				Type:     controlFrameDeviceDelta,
				Version:  controlProtocolVersion,
				Sequence: sequence,
			}, delta, frame)
			continue
		}
		s.enqueueControlFrame(sub, frame)
	}
}

func (s *ServerService) enqueueControlFrame(sub *serverControlConn, frame controlFrame) {
	s.enqueueControlMessage(sub, controlOutboundMessage{Frame: frame})
}

func (s *ServerService) enqueueControlPayload(sub *serverControlConn, frame controlFrame, payload any, fallback controlFrame) {
	rawPayload, err := marshalControlPayload(payload)
	if err != nil || len(rawPayload) > maxControlPayloadLength {
		s.enqueueControlFrame(sub, fallback)
		return
	}
	s.enqueueControlMessage(sub, controlOutboundMessage{Frame: frame, Payload: rawPayload})
}

func (s *ServerService) enqueueControlSnapshot(sub *serverControlConn, sequence uint64) {
	devices := s.buildDeviceStateV2()
	s.controlMu.Lock()
	s.controlState = deviceInfoV2Map(devices)
	s.controlMu.Unlock()
	s.enqueueControlPayload(sub, controlFrame{
		Type:     controlFrameDeviceSnapshot,
		Version:  controlProtocolVersion,
		Sequence: sequence,
	}, controlDeviceSnapshot{Sequence: sequence, Devices: devices}, controlFrame{
		Type:     controlFrameChanged,
		Version:  controlProtocolVersion,
		Sequence: sequence,
	})
}

func (s *ServerService) enqueueControlMessage(sub *serverControlConn, message controlOutboundMessage) {
	select {
	case sub.send <- message:
	default:
		s.logger.Debug("control subscriber ", sub.id, " lagged behind")
		_ = sub.conn.Close()
	}
}

func (s *ServerService) refreshControlState() {
	devices := s.buildDeviceStateV2()
	s.controlMu.Lock()
	s.controlState = deviceInfoV2Map(devices)
	s.controlMu.Unlock()
}

func (s *ServerService) buildDeviceStateV2() []DeviceInfoV2 {
	exports := s.currentExports()
	if len(exports) == 0 {
		return nil
	}
	devices := make([]DeviceInfoV2, 0, len(exports))
	for _, export := range exports {
		devices = append(devices, deviceInfoV2FromEntry(export.entry, "darwin-iokit", darwinStableID(export.registryID), deviceStateAvailable, 0, "available"))
	}
	return devices
}

func (s *ServerService) handleControlLeaseRequest(sub *serverControlConn, payload []byte) {
	var request controlLeaseRequest
	if err := unmarshalControlPayload(payload, &request); err != nil {
		s.enqueueControlPayload(sub, controlFrame{
			Type:    controlFrameLeaseResponse,
			Version: controlProtocolVersion,
		}, controlLeaseResponse{
			ErrorCode:    "bad_request",
			ErrorMessage: err.Error(),
		}, controlFrame{Type: controlFrameChanged, Version: controlProtocolVersion, Sequence: s.currentControlSequence()})
		return
	}
	response := s.createControlLeaseResponse(sub.id, request, s.darwinLeaseAvailable)
	s.enqueueControlPayload(sub, controlFrame{
		Type:    controlFrameLeaseResponse,
		Version: controlProtocolVersion,
	}, response, controlFrame{Type: controlFrameChanged, Version: controlProtocolVersion, Sequence: s.currentControlSequence()})
}

func (s *ServerService) darwinLeaseAvailable(busid string) (bool, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	export, ok := s.exports[busid]
	if !ok {
		return false, "unknown busid"
	}
	if export.busy {
		return false, "busy"
	}
	return true, ""
}

func (s *ServerService) createControlLeaseResponse(subID uint64, request controlLeaseRequest, available func(string) (bool, string)) controlLeaseResponse {
	response := controlLeaseResponse{
		BusID:       request.BusID,
		ClientNonce: request.ClientNonce,
	}
	if request.BusID == "" {
		response.ErrorCode = "bad_request"
		response.ErrorMessage = "missing busid"
		return response
	}
	if ok, reason := available(request.BusID); !ok {
		response.ErrorCode = "unavailable"
		response.ErrorMessage = reason
		return response
	}
	now := time.Now()
	expires := now.Add(importLeaseTTL)
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	s.cleanupExpiredImportLeasesLocked(now)
	if _, exists := s.leaseByBusID[request.BusID]; exists {
		response.ErrorCode = "busy"
		response.ErrorMessage = "lease already active"
		return response
	}
	s.leaseNextID++
	lease := serverImportLease{
		ID:           s.leaseNextID,
		SubscriberID: subID,
		BusID:        request.BusID,
		ClientNonce:  request.ClientNonce,
		Generation:   s.controlSeq,
		Expires:      expires,
	}
	if s.leases == nil {
		s.leases = make(map[uint64]serverImportLease)
	}
	if s.leaseByBusID == nil {
		s.leaseByBusID = make(map[string]uint64)
	}
	s.leases[lease.ID] = lease
	s.leaseByBusID[lease.BusID] = lease.ID
	response.LeaseID = lease.ID
	response.Generation = lease.Generation
	response.TTLMillis = int64(importLeaseTTL / time.Millisecond)
	return response
}

func (s *ServerService) consumeImportLease(request ImportExtRequest) bool {
	now := time.Now()
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	s.cleanupExpiredImportLeasesLocked(now)
	lease, ok := s.leases[request.LeaseID]
	if !ok {
		return false
	}
	if lease.BusID != request.BusID || lease.ClientNonce != request.ClientNonce {
		return false
	}
	delete(s.leases, request.LeaseID)
	delete(s.leaseByBusID, request.BusID)
	return now.Before(lease.Expires)
}

func (s *ServerService) cleanupExpiredImportLeasesLocked(now time.Time) {
	if s.leases == nil {
		s.leases = make(map[uint64]serverImportLease)
	}
	if s.leaseByBusID == nil {
		s.leaseByBusID = make(map[string]uint64)
	}
	for id, lease := range s.leases {
		if now.Before(lease.Expires) {
			continue
		}
		delete(s.leases, id)
		delete(s.leaseByBusID, lease.BusID)
	}
}

func (s *ServerService) deleteImportLeasesForSubscriberLocked(subID uint64) {
	for id, lease := range s.leases {
		if lease.SubscriberID != subID {
			continue
		}
		delete(s.leases, id)
		delete(s.leaseByBusID, lease.BusID)
	}
}

func (s *ServerService) currentControlSequence() uint64 {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	return s.controlSeq
}

func darwinStableID(registryID uint64) string {
	return "darwin-registry:" + hex32(uint32(registryID>>32)) + hex32(uint32(registryID))
}

type darwinServerDataSession struct {
	ctx     context.Context
	logger  log.ContextLogger
	conn    net.Conn
	device  *darwinUSBHostDevice
	writeMu sync.Mutex
	mu      sync.Mutex
	pending map[uint32]darwinServerPendingSubmit
	wg      sync.WaitGroup
}

type darwinServerPendingSubmit struct {
	endpoint uint8
	unlinked bool
}

func newDarwinServerDataSession(ctx context.Context, logger log.ContextLogger, conn net.Conn, device *darwinUSBHostDevice) *darwinServerDataSession {
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
	defer s.wg.Wait()
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
				s.writeMu.Lock()
				err := WriteSubmitResponse(s.conn, response)
				s.writeMu.Unlock()
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
				if err := s.device.abortEndpoint(endpoint); err != nil {
					s.logger.Debug("abort endpoint 0x", hex8(endpoint), ": ", err)
				}
				status = usbipStatusECONNRESET
			}
			s.writeMu.Lock()
			err = WriteUnlinkResponse(s.conn, UnlinkResponse{
				Header: DataHeader{Command: RetUnlink, SeqNum: header.SeqNum, DevID: header.DevID, Direction: header.Direction, Endpoint: header.Endpoint},
				Status: status,
			})
			s.writeMu.Unlock()
			if err != nil {
				return err
			}
		default:
			return E.New("unexpected USB/IP command 0x", hex32(header.Command))
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
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending[seq] = darwinServerPendingSubmit{endpoint: endpoint}
}

func (s *darwinServerDataSession) markSubmitUnlinked(seq uint32) (uint8, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pending, ok := s.pending[seq]
	if !ok {
		return 0, false
	}
	pending.unlinked = true
	s.pending[seq] = pending
	return pending.endpoint, true
}

func (s *darwinServerDataSession) finishSubmit(seq uint32) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	pending, ok := s.pending[seq]
	if !ok {
		return true
	}
	delete(s.pending, seq)
	return !pending.unlinked
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
