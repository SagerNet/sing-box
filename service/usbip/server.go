//go:build linux || (darwin && cgo)

package usbip

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strings"
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

// ServerService is the unified USB/IP server. It owns the wire-protocol
// surface (devlist, import, import-ext, control channel), the lease
// state machine, the broadcast bookkeeping, and the busy map.
// Platform-specific device acquisition is delegated to an ExportHost.
type ServerService struct {
	boxService.Adapter
	ctx      context.Context
	cancel   context.CancelFunc
	logger   log.ContextLogger
	listener *listener.Listener
	matches  []option.USBIPDeviceMatch
	host     ExportHost

	access  sync.Mutex
	exports map[string]Export
	busy    map[string]bool
	listen  net.Listener

	controlAccess sync.Mutex
	controlSeq    uint64
	controlNextID uint64
	controlSubs   map[uint64]*serverControlConn
	controlState  map[string]DeviceInfoV2
	leases        *LeaseManager

	reconcileAccess sync.Mutex
}

// NewServerService constructs a ServerService for the running platform.
func NewServerService(ctx context.Context, logger log.ContextLogger, tag string, options option.USBIPServerServiceOptions) (adapter.Service, error) {
	for i, m := range options.Devices {
		if m.IsZero() {
			return nil, E.New("devices[", i, "]: at least one of busid/vendor_id/product_id/serial is required")
		}
	}
	if options.ListenPort == 0 {
		options.ListenPort = DefaultPort
	}
	host, err := newPlatformExportHost(logger, options.Devices)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	return &ServerService{
		Adapter: boxService.NewAdapter(C.TypeUSBIPServer, tag),
		ctx:     ctx,
		cancel:  cancel,
		logger:  logger,
		matches: options.Devices,
		host:    host,
		exports: make(map[string]Export),
		busy:    make(map[string]bool),
		listener: listener.New(listener.Options{
			Context: ctx,
			Logger:  logger,
			Network: []string{N.NetworkTCP},
			Listen:  options.ListenOptions,
		}),
		controlSubs:  make(map[uint64]*serverControlConn),
		controlState: make(map[string]DeviceInfoV2),
		leases:       NewLeaseManager(importLeaseTTL, time.Now),
	}, nil
}

func (s *ServerService) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	err := s.host.Start(s.ctx)
	if err != nil {
		return err
	}
	err = s.reconcileAndBroadcast(false)
	if err != nil {
		_ = s.host.Close()
		return err
	}
	tcpListener, err := s.listener.ListenTCP()
	if err != nil {
		_ = s.host.Close()
		return err
	}
	s.access.Lock()
	s.listen = tcpListener
	s.access.Unlock()
	go s.acceptLoop(tcpListener)
	go s.eventLoop()
	return nil
}

func (s *ServerService) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
	s.closeControlSubscribers()
	err := common.Close(common.PtrOrNil(s.listener))
	s.reconcileAccess.Lock()
	defer s.reconcileAccess.Unlock()
	_ = s.host.Close()
	s.access.Lock()
	s.exports = make(map[string]Export)
	s.busy = make(map[string]bool)
	s.access.Unlock()
	return err
}

func (s *ServerService) eventLoop() {
	events, err := s.host.Events(s.ctx)
	if err != nil {
		s.logger.Warn("subscribe topology events: ", err)
		return
	}
	if events == nil {
		return
	}
	for {
		select {
		case <-s.ctx.Done():
			return
		case _, ok := <-events:
			if !ok {
				return
			}
		}
		err = s.reconcileAndBroadcast(true)
		if err != nil {
			s.logger.Warn("reconcile exports: ", err)
		}
	}
}

func (s *ServerService) reconcileAndBroadcast(notify bool) error {
	s.reconcileAccess.Lock()
	defer s.reconcileAccess.Unlock()
	if s.ctx != nil && s.ctx.Err() != nil {
		return nil
	}
	snapshot, released, _, err := s.host.Reconcile(s.ctx, s.isBusy)
	if err != nil {
		return err
	}
	s.access.Lock()
	s.exports = snapshot
	for _, busid := range released {
		delete(s.busy, busid)
	}
	s.access.Unlock()

	nextState := deviceInfoV2Map(s.buildDeviceStateV2())
	if notify {
		s.broadcastControlState(nextState, false)
	} else {
		s.setControlState(nextState)
	}
	return nil
}

func (s *ServerService) getExport(busid string) (Export, bool) {
	s.access.Lock()
	defer s.access.Unlock()
	export, ok := s.exports[busid]
	return export, ok
}

func (s *ServerService) isExported(busid string) bool {
	_, ok := s.getExport(busid)
	return ok
}

func (s *ServerService) isBusy(busid string) bool {
	s.access.Lock()
	defer s.access.Unlock()
	return s.busy[busid]
}

func (s *ServerService) setBusy(busid string, busy bool) bool {
	s.access.Lock()
	defer s.access.Unlock()
	current := s.busy[busid]
	if current == busy {
		return false
	}
	if busy {
		s.busy[busid] = true
	} else {
		delete(s.busy, busid)
	}
	return true
}

func (s *ServerService) removeExport(busid string) {
	s.access.Lock()
	defer s.access.Unlock()
	delete(s.exports, busid)
	delete(s.busy, busid)
}

func (s *ServerService) currentExports() []Export {
	s.access.Lock()
	defer s.access.Unlock()
	out := make([]Export, 0, len(s.exports))
	for busid, export := range s.exports {
		if s.busy[busid] {
			continue
		}
		out = append(out, export)
	}
	slices.SortFunc(out, exportLess)
	return out
}

func (s *ServerService) allExports() []Export {
	s.access.Lock()
	defer s.access.Unlock()
	out := make([]Export, 0, len(s.exports))
	for _, export := range s.exports {
		out = append(out, export)
	}
	slices.SortFunc(out, exportLess)
	return out
}

func exportLess(a, b Export) int {
	return strings.Compare(a.BusID(), b.BusID())
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
		s.handleDevList(conn)
	case OpReqImport:
		closeConn = !s.handleImport(conn)
	case OpReqImportExt:
		closeConn = !s.handleImportExt(conn)
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
	entries := s.buildDevListEntries()
	err := WriteOpRepDevList(conn, entries)
	if err != nil {
		s.logger.Debug("write devlist: ", err)
	}
}

func (s *ServerService) buildDevListEntries() []DeviceEntry {
	exports := s.currentExports()
	if len(exports) == 0 {
		return nil
	}
	entries := make([]DeviceEntry, 0, len(exports))
	for _, export := range exports {
		snapshot := export.Snapshot(s.ctx, false)
		if snapshot.Err != nil || snapshot.State != deviceStateAvailable {
			continue
		}
		entries = append(entries, snapshot.Entry)
	}
	return entries
}

func (s *ServerService) handleImport(conn net.Conn) bool {
	busid, err := ReadOpReqImportBody(conn)
	if err != nil {
		s.logger.Debug("read import body: ", err)
		return false
	}
	return s.handleImportBusID(conn, busid, false)
}

func (s *ServerService) handleImportExt(conn net.Conn) bool {
	request, err := ReadOpReqImportExtBody(conn)
	if err != nil {
		s.logger.Debug("read import-ext body: ", err)
		return false
	}
	if !s.consumeImportLease(request) {
		s.logger.Info("import-ext rejected (invalid lease): ", request.BusID)
		_ = WriteOpRepImportExt(conn, OpStatusError, nil)
		return false
	}
	return s.handleImportBusID(conn, request.BusID, true)
}

func (s *ServerService) handleImportBusID(conn net.Conn, busid string, extended bool) bool {
	writeReply := WriteOpRepImport
	if extended {
		writeReply = WriteOpRepImportExt
	}
	export, ok := s.getExport(busid)
	if !ok {
		s.logger.Info("import rejected (unknown busid): ", busid)
		_ = writeReply(conn, OpStatusError, nil)
		return false
	}
	if s.isBusy(busid) {
		s.logger.Info("import rejected (busid ", busid, " already busy)")
		_ = writeReply(conn, OpStatusError, nil)
		return false
	}
	ok, reason := export.LeaseCheck(s.ctx)
	if !ok {
		s.logger.Info("import rejected (busid ", busid, ": ", reason, ")")
		_ = writeReply(conn, OpStatusError, nil)
		return false
	}
	info, err := export.DeviceInfo(s.ctx)
	if err != nil {
		s.logger.Warn("refresh ", busid, ": ", err)
		_ = writeReply(conn, OpStatusError, nil)
		return false
	}
	session, err := export.NewServerDataSession(s.ctx, conn)
	if err != nil {
		s.logger.Warn("open data session ", busid, ": ", err)
		_ = writeReply(conn, OpStatusError, nil)
		return false
	}
	s.setBusy(busid, true)
	s.broadcastChanged()
	err = writeReply(conn, OpStatusOK, &info)
	if err != nil {
		s.logger.Warn("reply import ", busid, ": ", err)
		_ = session.Close()
		<-session.Done()
		_, _ = s.host.FinishImport(s.ctx, busid)
		s.setBusy(busid, false)
		s.broadcastChanged()
		return false
	}
	s.logger.Info("attached ", busid, " to remote ", conn.RemoteAddr())
	go s.waitImportDone(busid, session)
	return true
}

func (s *ServerService) waitImportDone(busid string, session DataSession) {
	<-session.Done()
	released, err := s.host.FinishImport(s.ctx, busid)
	if err != nil {
		s.logger.Debug("finish import ", busid, ": ", err)
	}
	if released {
		s.removeExport(busid)
		s.broadcastChanged()
		return
	}
	s.setBusy(busid, false)
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
		busid := export.BusID()
		snapshot := export.Snapshot(s.ctx, s.isBusy(busid))
		if snapshot.Err != nil {
			devices = append(devices, DeviceInfoV2{
				BusID:        busid,
				Backend:      snapshot.Backend,
				StableID:     snapshot.StableID,
				State:        deviceStateUnavailable,
				StatusReason: snapshot.StatusReason,
			})
			continue
		}
		// Skip exports that the host marked unavailable (e.g. Darwin
		// stale) — they should not appear in broadcast.
		if snapshot.State == deviceStateUnavailable && snapshot.Entry.Info.IDVendor == 0 {
			continue
		}
		devices = append(devices, deviceInfoV2FromEntry(snapshot.Entry, snapshot.Backend, snapshot.StableID, snapshot.State, snapshot.RawStatus, snapshot.StatusReason))
	}
	return devices
}

func (s *ServerService) leaseAvailable(busid string) (bool, string) {
	export, ok := s.getExport(busid)
	if !ok {
		return false, "unknown busid"
	}
	if s.isBusy(busid) {
		return false, deviceStateBusy
	}
	return export.LeaseCheck(s.ctx)
}
