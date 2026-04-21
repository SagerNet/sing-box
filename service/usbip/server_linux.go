//go:build linux

package usbip

import (
	"context"
	"net"
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
)

type serverExport struct {
	busid          string
	managed        bool
	originalDriver string
}

type ServerService struct {
	boxService.Adapter
	ctx      context.Context
	cancel   context.CancelFunc
	logger   log.ContextLogger
	listener *listener.Listener
	matches  []option.USBIPDeviceMatch

	mu       sync.Mutex
	exports  []serverExport
	listenFD net.Listener
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
		listener: listener.New(listener.Options{
			Context: ctx,
			Logger:  logger,
			Network: []string{N.NetworkTCP},
			Listen:  options.ListenOptions,
		}),
	}
	return s, nil
}

func (s *ServerService) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	if err := ensureHostDriver(); err != nil {
		return err
	}
	if err := s.bindExports(); err != nil {
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
	return nil
}

func (s *ServerService) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
	err := common.Close(common.PtrOrNil(s.listener))
	s.rollbackExports()
	return err
}

// bindExports resolves every match against current sysfs state, unbinds from
// the current driver, and binds to usbip-host.
func (s *ServerService) bindExports() error {
	devices, err := listUSBDevices()
	if err != nil {
		return E.Cause(err, "enumerate usb devices")
	}
	seen := make(map[string]bool)
	for _, m := range s.matches {
		matched := 0
		for i := range devices {
			if !Matches(m, devices[i].key()) {
				continue
			}
			if seen[devices[i].BusID] {
				matched++
				continue
			}
			if err := s.bindOne(&devices[i]); err != nil {
				s.logger.Warn("bind ", devices[i].BusID, ": ", err)
				continue
			}
			seen[devices[i].BusID] = true
			matched++
		}
		if matched == 0 {
			s.logger.Warn("no local device matched ", describeMatch(m))
		}
	}
	return nil
}

func (s *ServerService) bindOne(d *sysfsDevice) error {
	driver, err := currentDriver(d.BusID)
	if err != nil {
		return err
	}
	if driver == "usbip-host" {
		s.logger.Info("device ", d.BusID, " already bound to usbip-host; co-opting")
		s.mu.Lock()
		s.exports = append(s.exports, serverExport{busid: d.BusID})
		s.mu.Unlock()
		return nil
	}
	if driver != "" {
		if err := unbindFromDriver(d.BusID, driver); err != nil {
			return E.Cause(err, "unbind from ", driver)
		}
	}
	if err := hostMatchBusID(d.BusID, true); err != nil {
		if driver != "" {
			_ = bindToDriver(d.BusID, driver)
		}
		return E.Cause(err, "match_busid add")
	}
	if err := hostBind(d.BusID); err != nil {
		_ = hostMatchBusID(d.BusID, false)
		if driver != "" {
			_ = bindToDriver(d.BusID, driver)
		}
		return E.Cause(err, "bind to usbip-host")
	}
	s.logger.Info("exported ", d.BusID, " (previously on ", driverOrNone(driver), ")")
	s.mu.Lock()
	s.exports = append(s.exports, serverExport{
		busid:          d.BusID,
		managed:        true,
		originalDriver: driver,
	})
	s.mu.Unlock()
	return nil
}

func (s *ServerService) rollbackExports() {
	s.mu.Lock()
	exports := s.exports
	s.exports = nil
	s.mu.Unlock()
	for _, e := range exports {
		if !e.managed {
			continue
		}
		// Release any attached peer.
		_ = writeUsbipSockfd(e.busid, -1)
		if err := hostUnbind(e.busid); err != nil {
			s.logger.Warn("unbind ", e.busid, ": ", err)
		}
		if err := hostMatchBusID(e.busid, false); err != nil {
			s.logger.Debug("match_busid del ", e.busid, ": ", err)
		}
		if e.originalDriver == "" {
			s.logger.Info("released ", e.busid, " from usbip-host")
			continue
		}
		if err := bindToDriver(e.busid, e.originalDriver); err != nil {
			s.logger.Warn("rebind ", e.busid, " to ", e.originalDriver, ": ", err)
			continue
		}
		s.logger.Info("restored ", e.busid, " to ", e.originalDriver)
	}
}

func (s *ServerService) currentExports() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.exports))
	for i, e := range s.exports {
		out[i] = e.busid
	}
	return out
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
			s.logger.Error("accept: ", err)
			return
		}
		go s.handleConn(conn)
	}
}

func (s *ServerService) handleConn(conn net.Conn) {
	defer conn.Close()
	header, err := ReadOpHeader(conn)
	if err != nil {
		s.logger.Debug("read op header: ", err)
		return
	}
	switch header.Code {
	case OpReqDevList:
		s.handleDevList(conn)
	case OpReqImport:
		s.handleImport(conn)
	default:
		s.logger.Debug("unknown opcode 0x", hex16(header.Code))
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
		status, err := readUsbipStatus(busid)
		if err != nil {
			s.logger.Debug("status ", busid, ": ", err)
			continue
		}
		if status != usbipStatusAvailable {
			continue
		}
		d, err := readSysfsDevice(busid, sysBusDevicePath(busid))
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
	status, err := readUsbipStatus(busid)
	if err != nil || status != usbipStatusAvailable {
		s.logger.Info("import rejected (busid ", busid, " status=", status, " err=", err, ")")
		_ = WriteOpRepImport(conn, OpStatusError, nil)
		return
	}
	dev, err := readSysfsDevice(busid, sysBusDevicePath(busid))
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
	if err := writeUsbipSockfd(busid, int(file.Fd())); err != nil {
		s.logger.Warn("hand off ", busid, " to kernel: ", err)
		_ = WriteOpRepImport(conn, OpStatusError, nil)
		return
	}
	info := dev.toProtocol()
	if err := WriteOpRepImport(conn, OpStatusOK, &info); err != nil {
		s.logger.Warn("reply import ", busid, ": ", err)
		_ = writeUsbipSockfd(busid, -1)
		return
	}
	s.logger.Info("attached ", busid, " to remote ", conn.RemoteAddr())
}

func (s *ServerService) isExported(busid string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.exports {
		if e.busid == busid {
			return true
		}
	}
	return false
}

func sysBusDevicePath(busid string) string {
	return sysBusUSBDevices + "/" + busid
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
