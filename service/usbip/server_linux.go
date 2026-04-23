//go:build linux

package usbip

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
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
	"golang.org/x/sys/unix"
)

type serverExport struct {
	busid          string
	managed        bool
	originalDriver string
}

type serverControlConn struct {
	id   uint64
	conn net.Conn
	send chan controlFrame
}

type ServerService struct {
	boxService.Adapter
	ctx      context.Context
	cancel   context.CancelFunc
	logger   log.ContextLogger
	listener *listener.Listener
	matches  []option.USBIPDeviceMatch
	ops      usbipOps

	mu       sync.Mutex
	exports  map[string]serverExport
	listenFD net.Listener

	controlMu     sync.Mutex
	controlSeq    uint64
	controlNextID uint64
	controlSubs   map[uint64]*serverControlConn

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
	s := &ServerService{
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
		controlSubs: make(map[uint64]*serverControlConn),
		ops:         systemUSBIPOps,
	}
	return s, nil
}

func (s *ServerService) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	if err := s.ops.ensureHostDriver(); err != nil {
		return err
	}
	if err := s.reconcileAndBroadcast(false); err != nil {
		s.rollbackExports()
		return err
	}
	tcpListener, err := s.listener.ListenTCP()
	if err != nil {
		s.rollbackExports()
		return err
	}
	s.mu.Lock()
	s.listenFD = tcpListener
	s.mu.Unlock()
	go s.acceptLoop(tcpListener)
	go s.ueventLoop()
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
	devices, err := s.ops.listUSBDevices()
	if err != nil {
		return false, E.Cause(err, "enumerate usb devices")
	}
	desired := make(map[string]sysfsDevice)
	present := make(map[string]struct{}, len(devices))
	for i := range devices {
		present[devices[i].BusID] = struct{}{}
	}
	for _, m := range s.matches {
		for i := range devices {
			if !Matches(m, devices[i].key()) {
				continue
			}
			if isVHCIImportedDevice(devices[i].Path) {
				s.logger.Debug("skip vhci-imported device ", devices[i].BusID, " matched by ", describeMatch(m))
				continue
			}
			if devices[i].DeviceClass == 0x09 {
				s.logger.Warn("skip hub device ", devices[i].BusID, " matched by ", describeMatch(m))
				continue
			}
			desired[devices[i].BusID] = devices[i]
		}
	}

	current := s.snapshotExports()
	changed := false
	for busid, device := range desired {
		if _, ok := current[busid]; ok {
			continue
		}
		if err := s.bindOne(&device); err != nil {
			s.logger.Warn("bind ", busid, ": ", err)
			continue
		}
		changed = true
	}
	for busid, export := range current {
		if _, ok := desired[busid]; ok {
			continue
		}
		_, restore := present[busid]
		if err := s.releaseExport(export, restore); err != nil {
			s.logger.Warn("release ", busid, ": ", err)
		}
		changed = true
	}
	return changed, nil
}

func (s *ServerService) bindOne(d *sysfsDevice) error {
	driver, err := s.ops.currentDriver(d.BusID)
	if err != nil {
		return err
	}
	if driver == "usbip-host" {
		s.logger.Info("device ", d.BusID, " already bound to usbip-host; co-opting")
		s.setExport(serverExport{busid: d.BusID})
		return nil
	}
	if driver != "" {
		if err := s.ops.unbindFromDriver(d.BusID, driver); err != nil {
			return E.Cause(err, "unbind from ", driver)
		}
	}
	if err := s.ops.hostMatchBusID(d.BusID, true); err != nil {
		if driver != "" {
			_ = s.ops.bindToDriver(d.BusID, driver)
		}
		return E.Cause(err, "match_busid add")
	}
	if err := s.ops.hostBind(d.BusID); err != nil {
		_ = s.ops.hostMatchBusID(d.BusID, false)
		if driver != "" {
			_ = s.ops.bindToDriver(d.BusID, driver)
		}
		return E.Cause(err, "bind to usbip-host")
	}
	s.logger.Info("exported ", d.BusID, " (previously on ", driverOrNone(driver), ")")
	s.setExport(serverExport{
		busid:          d.BusID,
		managed:        true,
		originalDriver: driver,
	})
	return nil
}

func (s *ServerService) releaseExport(export serverExport, restore bool) error {
	if !export.managed {
		s.deleteExport(export.busid)
		s.logger.Info("stopped tracking ", export.busid, " on usbip-host")
		return nil
	}
	if err := s.ops.writeUsbipSockfd(export.busid, -1); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := s.ops.hostUnbind(export.busid); err != nil && !os.IsNotExist(err) && !(isMissingUSBDeviceError(err) && !restore) {
		return err
	}
	if err := s.ops.hostMatchBusID(export.busid, false); err != nil {
		return err
	}
	if !restore {
		s.deleteExport(export.busid)
		s.logger.Info("removed export state for disappeared device ", export.busid)
		return nil
	}
	if export.originalDriver == "" {
		s.deleteExport(export.busid)
		s.logger.Info("released ", export.busid, " from usbip-host")
		return nil
	}
	if err := s.ops.bindToDriver(export.busid, export.originalDriver); err != nil {
		return err
	}
	s.deleteExport(export.busid)
	s.logger.Info("restored ", export.busid, " to ", export.originalDriver)
	return nil
}

func (s *ServerService) rollbackExports() {
	exports := s.snapshotExports()
	for _, export := range exports {
		_, err := s.ops.readSysfsDevice(export.busid, sysBusDevicePath(export.busid))
		restore := err == nil
		if err := s.releaseExport(export, restore); err != nil {
			s.logger.Warn("rollback ", export.busid, ": ", err)
		}
	}
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
	}
	return nil
}

func (s *ServerService) currentExports() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.exports))
	for busid := range s.exports {
		out = append(out, busid)
	}
	slices.Sort(out)
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
			//nolint:staticcheck
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
	default:
		s.logger.Debug("unknown opcode 0x", hex16(header.Code))
	}
}

func (s *ServerService) handleControlConn(conn net.Conn) {
	defer conn.Close()

	hello, err := ReadControlFrame(conn)
	if err != nil {
		s.logger.Debug("read control hello: ", err)
		return
	}
	if hello.Type != controlFrameHello {
		s.logger.Debug("unexpected control frame ", hello.Type, " before hello")
		return
	}
	if hello.Version != controlProtocolVersion {
		s.logger.Debug("unsupported control version ", hello.Version)
		return
	}
	if hello.Capabilities&controlCapabilities != controlCapabilities {
		s.logger.Debug("missing control capabilities 0x", hello.Capabilities)
		return
	}

	sub, seq := s.registerControlConn(conn)
	defer s.unregisterControlConn(sub.id)

	if err := WriteControlAck(conn, seq); err != nil {
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
		case frame := <-sub.send:
			if err := writeControlFrame(conn, frame); err != nil {
				s.logger.Debug("write control frame: ", err)
				return
			}
		}
	}
}

func (s *ServerService) readControlConn(sub *serverControlConn, done chan<- struct{}) {
	defer close(done)
	for {
		frame, err := ReadControlFrame(sub.conn)
		if err != nil {
			return
		}
		switch frame.Type {
		case controlFramePing:
			s.enqueueControlFrame(sub, controlFrame{
				Type:    controlFramePong,
				Version: controlProtocolVersion,
			})
		default:
			return
		}
	}
}

func (s *ServerService) handleDevList(conn net.Conn) {
	entries := s.buildDevListEntries()
	if err := WriteOpRepDevList(conn, entries); err != nil {
		s.logger.Debug("write devlist: ", err)
	}
}

func (s *ServerService) buildDevListEntries() []DeviceEntry {
	busids := s.currentExports()
	if len(busids) == 0 {
		return nil
	}
	entries := make([]DeviceEntry, 0, len(busids))
	for _, busid := range busids {
		status, err := s.ops.readUsbipStatus(busid)
		if err != nil {
			s.logger.Debug("status ", busid, ": ", err)
			continue
		}
		if status != usbipStatusAvailable {
			continue
		}
		d, err := s.ops.readSysfsDevice(busid, sysBusDevicePath(busid))
		if err != nil {
			s.logger.Debug("refresh ", busid, ": ", err)
			continue
		}
		entries = append(entries, DeviceEntry{
			Info:       d.toProtocol(),
			Interfaces: d.Interfaces,
		})
	}
	return entries
}

func (s *ServerService) handleImport(conn net.Conn) {
	busid, err := ReadOpReqImportBody(conn)
	if err != nil {
		s.logger.Debug("read import body: ", err)
		return
	}
	if !s.isExported(busid) {
		s.logger.Info("import rejected (unknown busid): ", busid)
		_ = WriteOpRepImport(conn, OpStatusError, nil)
		return
	}
	status, err := s.ops.readUsbipStatus(busid)
	if err != nil || status != usbipStatusAvailable {
		s.logger.Info("import rejected (busid ", busid, " status=", status, " err=", err, ")")
		_ = WriteOpRepImport(conn, OpStatusError, nil)
		return
	}
	dev, err := s.ops.readSysfsDevice(busid, sysBusDevicePath(busid))
	if err != nil {
		s.logger.Warn("refresh ", busid, ": ", err)
		_ = WriteOpRepImport(conn, OpStatusError, nil)
		return
	}
	tcp, ok := conn.(*net.TCPConn)
	if !ok {
		s.logger.Warn("import requires *net.TCPConn, got ", conn)
		_ = WriteOpRepImport(conn, OpStatusError, nil)
		return
	}
	file, err := tcp.File()
	if err != nil {
		s.logger.Warn("dup socket fd: ", err)
		_ = WriteOpRepImport(conn, OpStatusError, nil)
		return
	}
	defer file.Close()
	if err := s.ops.writeUsbipSockfd(busid, int(file.Fd())); err != nil {
		s.logger.Warn("hand off ", busid, " to kernel: ", err)
		_ = WriteOpRepImport(conn, OpStatusError, nil)
		return
	}
	info := dev.toProtocol()
	if err := WriteOpRepImport(conn, OpStatusOK, &info); err != nil {
		s.logger.Warn("reply import ", busid, ": ", err)
		_ = s.ops.writeUsbipSockfd(busid, -1)
		return
	}
	s.logger.Info("attached ", busid, " to remote ", conn.RemoteAddr())
}

func (s *ServerService) isExported(busid string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.exports[busid]
	return ok
}

func (s *ServerService) ueventLoop() {
	for {
		listener, err := s.ops.newUEventListener()
		if err != nil {
			if s.ctx.Err() != nil {
				return
			}
			s.logger.Warn("open uevent listener: ", err)
			if !sleepCtx(s.ctx, time.Second) {
				return
			}
			continue
		}
		done := make(chan struct{})
		go func() {
			select {
			case <-s.ctx.Done():
				_ = listener.Close()
			case <-done:
			}
		}()
		for {
			err = listener.WaitUSBEvent()
			if err != nil {
				close(done)
				_ = listener.Close()
				if s.ctx.Err() != nil {
					return
				}
				s.logger.Warn("read uevent: ", err)
				if !sleepCtx(s.ctx, time.Second) {
					return
				}
				break
			}
			if err := s.reconcileAndBroadcast(true); err != nil {
				s.logger.Warn("reconcile exports: ", err)
			}
		}
	}
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

func (s *ServerService) registerControlConn(conn net.Conn) (*serverControlConn, uint64) {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	s.controlNextID++
	sub := &serverControlConn{
		id:   s.controlNextID,
		conn: conn,
		send: make(chan controlFrame, 16),
	}
	s.controlSubs[sub.id] = sub
	return sub, s.controlSeq
}

func (s *ServerService) unregisterControlConn(id uint64) {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	delete(s.controlSubs, id)
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
	s.controlMu.Lock()
	s.controlSeq++
	sequence := s.controlSeq
	subs := make([]*serverControlConn, 0, len(s.controlSubs))
	for _, sub := range s.controlSubs {
		subs = append(subs, sub)
	}
	s.controlMu.Unlock()

	frame := controlFrame{
		Type:     controlFrameChanged,
		Version:  controlProtocolVersion,
		Sequence: sequence,
	}
	for _, sub := range subs {
		s.enqueueControlFrame(sub, frame)
	}
}

func (s *ServerService) enqueueControlFrame(sub *serverControlConn, frame controlFrame) {
	select {
	case sub.send <- frame:
	default:
		s.logger.Debug("control subscriber ", sub.id, " lagged behind")
		_ = sub.conn.Close()
	}
}

func sysBusDevicePath(busid string) string {
	return sysBusUSBDevices + "/" + busid
}

func isVHCIImportedDevice(path string) bool {
	if strings.Contains(path, "vhci_hcd") {
		return true
	}
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	return strings.Contains(realPath, "vhci_hcd")
}

func isMissingUSBDeviceError(err error) bool {
	return errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ENODEV)
}

func describeMatch(m option.USBIPDeviceMatch) string {
	var parts []string
	if m.BusID != "" {
		parts = append(parts, "busid="+m.BusID)
	}
	if m.VendorID != 0 {
		parts = append(parts, "vendor_id=0x"+hex16(uint16(m.VendorID)))
	}
	if m.ProductID != 0 {
		parts = append(parts, "product_id=0x"+hex16(uint16(m.ProductID)))
	}
	if m.Serial != "" {
		parts = append(parts, "serial="+m.Serial)
	}
	return "{" + joinComma(parts) + "}"
}

func driverOrNone(d string) string {
	if d == "" {
		return "(no driver)"
	}
	return d
}

func hex16(v uint16) string {
	const hexdigits = "0123456789abcdef"
	return string([]byte{
		hexdigits[(v>>12)&0xf],
		hexdigits[(v>>8)&0xf],
		hexdigits[(v>>4)&0xf],
		hexdigits[v&0xf],
	})
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ","
		}
		out += p
	}
	return out
}
