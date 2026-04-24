//go:build linux

package usbip

import (
	"context"
	"errors"
	"fmt"
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

const (
	usbipExportReleaseTimeout      = 10 * time.Second
	usbipExportReleasePollInterval = 100 * time.Millisecond
	serverReconcileBackstop        = 30 * time.Second
)

type ServerService struct {
	boxService.Adapter
	ctx      context.Context
	cancel   context.CancelFunc
	logger   log.ContextLogger
	listener *listener.Listener
	matches  []option.USBIPDeviceMatch
	ops      usbipOps

	access  sync.Mutex
	exports map[string]serverExport
	listen  net.Listener

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
		controlSubs:   make(map[uint64]*serverControlConn),
		controlState:  make(map[string]DeviceInfoV2),
		leasesByBusID: make(map[string]serverImportLease),
		ops:           systemUSBIPOps,
	}
	return s, nil
}

func (s *ServerService) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	err := s.ops.ensureHostDriver()
	if err != nil {
		return err
	}
	err = s.reconcileAndBroadcast(false)
	if err != nil {
		s.rollbackExports()
		return err
	}
	var tcpListener net.Listener
	tcpListener, err = s.listener.ListenTCP()
	if err != nil {
		s.rollbackExports()
		return err
	}
	s.access.Lock()
	s.listen = tcpListener
	s.access.Unlock()
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
	s.reconcileAccess.Lock()
	defer s.reconcileAccess.Unlock()
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
			if !matches(m, devices[i].key()) {
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
		err = s.bindOne(&device)
		if err != nil {
			return changed, E.Cause(err, "bind ", busid)
		}
		changed = true
	}
	for busid, export := range current {
		if _, ok := desired[busid]; ok {
			continue
		}
		_, restore := present[busid]
		err = s.releaseExport(export, restore)
		if err != nil {
			s.logger.Warn("release ", busid, ": ", err)
		}
		changed = true
	}
	return changed, nil
}

func (s *ServerService) bindOne(d *sysfsDevice) error {
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		err = s.bindOneOnce(d)
		if err == nil {
			return nil
		}
		if attempt > 0 || !errors.Is(err, unix.ENODEV) {
			break
		}
		s.logger.Warn("reset usbip-host after bind failure on ", d.BusID, ": ", err)
		if resetErr := s.resetHostDriverForBindRetry(); resetErr != nil {
			return E.Cause(resetErr, "reset usbip-host after bind failure")
		}
	}
	return err
}

func (s *ServerService) bindOneOnce(d *sysfsDevice) error {
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
		err = s.ops.unbindFromDriver(d.BusID, driver)
		if err != nil {
			return E.Cause(err, "unbind from ", driver)
		}
	}
	err = s.ops.hostMatchBusID(d.BusID, true)
	if err != nil {
		if driver != "" {
			_ = s.ops.bindToDriver(d.BusID, driver)
		}
		return E.Cause(err, "match_busid add")
	}
	err = s.ops.hostBind(d.BusID)
	if err != nil {
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

func (s *ServerService) resetHostDriverForBindRetry() error {
	if len(s.snapshotExports()) > 0 {
		return E.New("active usbip-host exports are present")
	}
	return s.ops.reloadHostDriver()
}

func (s *ServerService) releaseExport(export serverExport, restore bool) error {
	if !export.managed {
		s.deleteExport(export.busid)
		s.logger.Info("stopped tracking ", export.busid, " on usbip-host")
		return nil
	}
	status, statusErr := s.ops.readUsbipStatus(export.busid)
	if statusErr != nil && !os.IsNotExist(statusErr) && !isMissingUSBDeviceError(statusErr) {
		return statusErr
	}
	if statusErr == nil && status == usbipStatusUsed {
		err := s.ops.writeUsbipSockfd(export.busid, -1)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if restore {
			err = s.waitUSBIPStatusAvailable(export.busid, usbipExportReleaseTimeout)
			if err != nil {
				return err
			}
		}
	}
	err := s.ops.hostUnbind(export.busid)
	if err != nil && !os.IsNotExist(err) && !(isMissingUSBDeviceError(err) && !restore) {
		return err
	}
	err = s.ops.hostMatchBusID(export.busid, false)
	if err != nil {
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
	err = s.ops.bindToDriver(export.busid, export.originalDriver)
	if err != nil {
		return err
	}
	s.deleteExport(export.busid)
	s.logger.Info("restored ", export.busid, " to ", export.originalDriver)
	return nil
}

func (s *ServerService) waitUSBIPStatusAvailable(busid string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		status, err := s.ops.readUsbipStatus(busid)
		if err != nil {
			if os.IsNotExist(err) || isMissingUSBDeviceError(err) {
				return nil
			}
		} else if status == usbipStatusAvailable {
			return nil
		}
		if time.Now().After(deadline) {
			if err != nil {
				return E.Cause(err, "wait for ", busid, " usbip status available")
			}
			return E.New("timed out waiting for ", busid, " usbip status available")
		}
		if !sleepCtx(s.ctx, usbipExportReleasePollInterval) {
			return s.ctx.Err()
		}
	}
}

func (s *ServerService) rollbackExports() {
	exports := s.snapshotExports()
	for _, export := range exports {
		_, err := s.ops.readSysfsDevice(export.busid, sysBusDevicePath(export.busid))
		restore := err == nil
		err = s.releaseExport(export, restore)
		if err != nil {
			s.logger.Warn("rollback ", export.busid, ": ", err)
		}
	}
}

func (s *ServerService) reconcileAndBroadcast(notify bool) error {
	s.reconcileAccess.Lock()
	defer s.reconcileAccess.Unlock()

	if s.ctx != nil && s.ctx.Err() != nil {
		return nil
	}
	if _, err := s.reconcileExports(); err != nil {
		return err
	}

	nextState := deviceInfoV2Map(s.buildDeviceStateV2())
	if notify {
		s.broadcastControlState(nextState, false)
	} else {
		s.setControlState(nextState)
	}
	return nil
}

func (s *ServerService) currentExports() []string {
	s.access.Lock()
	defer s.access.Unlock()
	out := make([]string, 0, len(s.exports))
	for busid := range s.exports {
		out = append(out, busid)
	}
	slices.Sort(out)
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
		entries = append(entries, d.toDeviceEntry())
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
	if !s.isExported(busid) {
		s.logger.Info("import rejected (unknown busid): ", busid)
		_ = writeReply(conn, OpStatusError, nil)
		return false
	}
	status, err := s.ops.readUsbipStatus(busid)
	if err != nil || status != usbipStatusAvailable {
		s.logger.Info("import rejected (busid ", busid, " status=", status, " err=", err, ")")
		_ = writeReply(conn, OpStatusError, nil)
		return false
	}
	dev, err := s.ops.readSysfsDevice(busid, sysBusDevicePath(busid))
	if err != nil {
		s.logger.Warn("refresh ", busid, ": ", err)
		_ = writeReply(conn, OpStatusError, nil)
		return false
	}
	handoff, err := newUSBIPConnHandoff(conn)
	if err != nil {
		s.logger.Warn("prepare handoff ", busid, ": ", err)
		_ = writeReply(conn, OpStatusError, nil)
		return false
	}
	defer handoff.Close()
	s.logger.Debug("usbip server handoff ", busid, ": ", handoff.mode())
	err = s.ops.writeUsbipSockfd(busid, int(handoff.kernelFD()))
	if err != nil {
		s.logger.Warn("hand off ", busid, " to kernel: ", err)
		_ = writeReply(conn, OpStatusError, nil)
		return false
	}
	err = handoff.closeKernelFD()
	if err != nil {
		s.logger.Debug("close kernel fd ", busid, ": ", err)
	}
	info := dev.toProtocol()
	err = writeReply(conn, OpStatusOK, &info)
	if err != nil {
		s.logger.Warn("reply import ", busid, ": ", err)
		_ = s.ops.writeUsbipSockfd(busid, -1)
		return false
	}
	s.logger.Info("attached ", busid, " to remote ", conn.RemoteAddr())
	return handoff.startRelay(s.ctx, s.logger, "server", busid)
}

func (s *ServerService) isExported(busid string) bool {
	s.access.Lock()
	defer s.access.Unlock()
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
			err := s.reconcileAndBroadcast(true)
			if err != nil {
				s.logger.Warn("reconcile exports: ", err)
			}
		}
	}
}

func (s *ServerService) reconcileLoop() {
	ticker := time.NewTicker(serverReconcileBackstop)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
		}

		err := s.reconcileAndBroadcast(true)
		if err != nil {
			s.logger.Warn("reconcile exports: ", err)
		}
	}
}

func (s *ServerService) buildDeviceStateV2() []DeviceInfoV2 {
	busids := s.currentExports()
	if len(busids) == 0 {
		return nil
	}
	devices := make([]DeviceInfoV2, 0, len(busids))
	for _, busid := range busids {
		status, statusErr := s.ops.readUsbipStatus(busid)
		dev, devErr := s.ops.readSysfsDevice(busid, sysBusDevicePath(busid))
		if devErr != nil {
			devices = append(devices, DeviceInfoV2{
				BusID:        busid,
				Backend:      backendIDLinuxSysfs,
				StableID:     linuxStableID(sysfsDevice{BusID: busid}),
				State:        deviceStateUnavailable,
				StatusReason: devErr.Error(),
			})
			continue
		}
		state := linuxUSBIPStatusState(status)
		reason := linuxUSBIPStatusReason(status)
		if statusErr != nil {
			state = deviceStateUnavailable
			reason = statusErr.Error()
		}
		entry := dev.toDeviceEntry()
		devices = append(devices, deviceInfoV2FromEntry(entry, backendIDLinuxSysfs, linuxStableID(dev), state, status, reason))
	}
	return devices
}

func (s *ServerService) leaseAvailable(busid string) (bool, string) {
	if !s.isExported(busid) {
		return false, "unknown busid"
	}
	status, err := s.ops.readUsbipStatus(busid)
	if err != nil {
		return false, err.Error()
	}
	if status != usbipStatusAvailable {
		return false, linuxUSBIPStatusReason(status)
	}
	return true, ""
}

func linuxStableID(d sysfsDevice) string {
	if d.Serial != "" {
		return fmt.Sprintf("usb:%04x:%04x:%s", d.VendorID, d.ProductID, d.Serial)
	}
	return "linux-busid:" + d.BusID
}

func linuxUSBIPStatusState(status int) string {
	switch status {
	case usbipStatusAvailable:
		return deviceStateAvailable
	case usbipStatusUsed:
		return deviceStateBusy
	default:
		return deviceStateUnavailable
	}
}

func linuxUSBIPStatusReason(status int) string {
	switch status {
	case usbipStatusAvailable:
		return "available"
	case usbipStatusUsed:
		return "used"
	case usbipStatusError:
		return "error"
	default:
		return fmt.Sprintf("status=0x%08x", uint32(status))
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

func driverOrNone(d string) string {
	if d == "" {
		return "(no driver)"
	}
	return d
}
