//go:build linux || (darwin && cgo) || windows

package usbip

import (
	"context"
	"fmt"
	"net"
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
)

type ServerService struct {
	boxService.Adapter
	ctx      context.Context
	cancel   context.CancelFunc
	logger   log.ContextLogger
	listener *listener.Listener
	matches  []option.USBIPDeviceMatch
	host     ExportHost
	ledger   *exportLedger

	reconcileAccess sync.Mutex

	sessionsAccess sync.Mutex
	sessions       map[DataSession]struct{}
	sessionsClosed bool
}

func NewServerService(ctx context.Context, logger log.ContextLogger, tag string, options option.USBIPServerServiceOptions) (adapter.Service, error) {
	if len(options.Devices) == 0 {
		return nil, E.New("devices: at least one match is required")
	}
	for i, m := range options.Devices {
		if m.IsZero() {
			return nil, E.New("devices[", i, "]: at least one of busid/vendor_id/product_id/serial is required")
		}
	}
	if options.ListenPort == 0 {
		options.ListenPort = DefaultPort
	}
	ctx, cancel := context.WithCancel(ctx)
	host, err := newPlatformExportHost(ctx, logger, options.Devices)
	if err != nil {
		cancel()
		return nil, err
	}
	return &ServerService{
		Adapter:  boxService.NewAdapter(C.TypeUSBIPServer, tag),
		ctx:      ctx,
		cancel:   cancel,
		logger:   logger,
		matches:  options.Devices,
		host:     host,
		ledger:   newExportLedger(logger, importLeaseTTL, time.Now),
		sessions: make(map[DataSession]struct{}),
		listener: listener.New(listener.Options{
			Context: ctx,
			Logger:  logger,
			Network: []string{N.NetworkTCP},
			Listen:  options.ListenOptions,
		}),
	}, nil
}

func (s *ServerService) Start(stage adapter.StartStage) (err error) {
	if stage != adapter.StartStateStart {
		return nil
	}
	defer func() {
		if err != nil {
			s.cancel()
			_ = s.host.Close()
		}
	}()
	err = s.host.Start()
	if err != nil {
		return err
	}
	events, err := s.host.Events()
	if err != nil {
		return E.Cause(err, "subscribe topology events")
	}
	err = s.reconcileAndBroadcast(false)
	if err != nil {
		return err
	}
	tcpListener, err := s.listener.ListenTCP()
	if err != nil {
		return err
	}
	go s.acceptLoop(tcpListener)
	go s.eventLoop(events)
	return nil
}

func (s *ServerService) Close() error {
	if s.cancel != nil {
		s.cancel()
	}

	s.sessionsAccess.Lock()
	s.sessionsClosed = true
	sessions := make([]DataSession, 0, len(s.sessions))
	for session := range s.sessions {
		sessions = append(sessions, session)
	}
	s.sessionsAccess.Unlock()

	for _, conn := range s.ledger.CloseAllSubscribers() {
		_ = conn.Close()
	}
	err := common.Close(common.PtrOrNil(s.listener))

	for _, session := range sessions {
		_ = session.Close()
	}

	s.reconcileAccess.Lock()
	defer s.reconcileAccess.Unlock()
	_ = s.host.Close()
	s.ledger.ResetForClose()
	return err
}

func (s *ServerService) eventLoop(events <-chan struct{}) {
	for {
		select {
		case <-s.ctx.Done():
			return
		case _, ok := <-events:
			if !ok {
				return
			}
		}
		err := s.reconcileAndBroadcast(true)
		if err != nil {
			s.logger.Warn("reconcile exports: ", err)
		}
	}
}

func (s *ServerService) tearDownPreparedSession(busid string, session DataSession) {
	_ = session.Close()
	<-session.Done()
	released, _ := s.host.FinishImport(busid)
	s.ledger.ReleaseImport(busid, released)
	if released {
		err := s.reconcileAndBroadcast(true)
		if err != nil {
			s.logger.Debug("reconcile after ", busid, ": ", err)
		}
	}
}

func (s *ServerService) reconcileAndBroadcast(notify bool) error {
	s.reconcileAccess.Lock()
	defer s.reconcileAccess.Unlock()
	if s.ctx.Err() != nil {
		return nil
	}
	snapshot, released, err := s.host.Reconcile(s.ledger.IsReserved)
	s.ledger.ApplyHostSnapshot(snapshot, released)
	if notify {
		s.ledger.BroadcastIfChanged()
	} else {
		s.ledger.SeedBroadcastState()
	}
	return err
}

func (s *ServerService) handleStandardConn(conn net.Conn, header OpHeader) {
	closeConn := true
	defer func() {
		if closeConn {
			_ = conn.Close()
		}
	}()
	switch header.Code {
	case OpReqDevList:
		entries := s.buildDevListEntries()
		err := WriteOpRepDevList(conn, entries)
		if err != nil {
			s.logger.Debug("write devlist: ", err)
		}
	case OpReqImport:
		busid, err := ReadOpReqImportBody(conn)
		if err != nil {
			s.logger.Debug("read import body: ", err)
			break
		}
		closeConn = !s.handleImportBusID(conn, busid)
	case OpReqImportExt:
		closeConn = !s.handleImportExt(conn)
	default:
		s.logger.Debug(fmt.Sprintf("unknown opcode 0x%04x", header.Code))
	}
}

func (s *ServerService) handleControlConn(conn net.Conn) {
	defer conn.Close()
	var cr controlReader
	helloMessage, err := cr.read(conn)
	if err != nil {
		s.logger.Debug("read control hello: ", err)
		return
	}
	hello := helloMessage.Frame
	if hello.Type != controlFrameHello {
		s.logger.Debug("unexpected control frame ", hello.Type, " before hello")
		return
	}
	if hello.Version != controlProtocolVersion {
		s.logger.Debug("unsupported control version ", hello.Version)
		return
	}
	if hello.Capabilities&controlRequiredCapabilities != controlRequiredCapabilities {
		s.logger.Debug("missing control capabilities 0x", hello.Capabilities)
		return
	}
	capabilities := hello.Capabilities & controlCapabilities
	sub, seq := s.ledger.Subscribe(conn, capabilities)
	defer s.ledger.Unsubscribe(sub)
	err = writeControlMessage(conn, controlFrame{
		Type:         controlFrameAck,
		Version:      controlProtocolVersion,
		Capabilities: capabilities,
		Sequence:     seq,
	}, nil)
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

func (s *ServerService) buildDevListEntries() []DeviceEntry {
	exports := s.ledger.AvailableExports()
	if len(exports) == 0 {
		return nil
	}
	entries := make([]DeviceEntry, 0, len(exports))
	for _, export := range exports {
		snapshot := export.Snapshot(false)
		if snapshot.State != deviceStateAvailable {
			continue
		}
		entries = append(entries, snapshot.Entry)
	}
	return entries
}

func (s *ServerService) handleImportExt(conn net.Conn) bool {
	request, err := ReadOpReqImportExtBody(conn)
	if err != nil {
		s.logger.Debug("read import-ext body: ", err)
		return false
	}
	export, ok, reason := s.ledger.ConsumeLeaseAndReserve(request)
	if !ok {
		s.logger.Info("import-ext rejected (", request.BusID, ": ", reason, ")")
		_ = WriteOpRepImport(conn, OpRepImportExt, OpStatusError, nil)
		return false
	}
	return s.handleImportReserved(conn, request.BusID, export, true)
}

func (s *ServerService) handleImportBusID(conn net.Conn, busid string) bool {
	export, ok, reason := s.ledger.TryReserveForImport(busid)
	if !ok {
		s.logger.Info("import rejected (", busid, ": ", reason, ")")
		_ = WriteOpRepImport(conn, OpRepImport, OpStatusError, nil)
		return false
	}
	return s.handleImportReserved(conn, busid, export, false)
}

func (s *ServerService) handleImportReserved(conn net.Conn, busid string, export Export, extended bool) bool {
	opCode := uint16(OpRepImport)
	if extended {
		opCode = OpRepImportExt
	}
	info, err := export.DeviceInfo()
	if err != nil {
		s.ledger.ReleaseImport(busid, false)
		s.logger.Warn("refresh ", busid, ": ", err)
		_ = WriteOpRepImport(conn, opCode, OpStatusError, nil)
		return false
	}
	session, err := export.NewServerDataSession(s.ctx, conn)
	if err != nil {
		s.ledger.ReleaseImport(busid, false)
		s.logger.Warn("open data session ", busid, ": ", err)
		_ = WriteOpRepImport(conn, opCode, OpStatusError, nil)
		return false
	}
	s.ledger.BroadcastIfChanged()
	err = WriteOpRepImport(conn, opCode, OpStatusOK, &info)
	if err != nil {
		s.logger.Warn("reply import ", busid, ": ", err)
		s.tearDownPreparedSession(busid, session)
		return false
	}

	s.sessionsAccess.Lock()
	if s.sessionsClosed {
		s.sessionsAccess.Unlock()
		s.tearDownPreparedSession(busid, session)
		return false
	}
	// Close may observe a prepared session before Start runs, so
	// DataSession implementations must treat Close-before-Start as valid.
	s.sessions[session] = struct{}{}
	s.sessionsAccess.Unlock()

	err = session.Start()
	if err != nil {
		s.sessionsAccess.Lock()
		delete(s.sessions, session)
		s.sessionsAccess.Unlock()
		s.logger.Warn("start data session ", busid, ": ", err)
		s.tearDownPreparedSession(busid, session)
		return false
	}
	s.logger.Info("attached ", busid, " to remote ", conn.RemoteAddr())
	go func() {
		<-session.Done()
		s.sessionsAccess.Lock()
		delete(s.sessions, session)
		s.sessionsAccess.Unlock()
		released, err := s.host.FinishImport(busid)
		if err != nil {
			s.logger.Debug("finish import ", busid, ": ", err)
		}
		s.ledger.ReleaseImport(busid, released)
		if released {
			err = s.reconcileAndBroadcast(true)
			if err != nil {
				s.logger.Debug("reconcile after ", busid, ": ", err)
			}
		}
	}()
	return true
}
