//go:build linux

package usbip

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/unix"
)

func newPlatformExportHost(logger log.ContextLogger, matches []option.USBIPDeviceMatch) (ExportHost, error) {
	return newLinuxExportHost(logger, matches), nil
}

func newPlatformImportHost(logger log.ContextLogger) (ImportHost, error) {
	return &linuxImportHost{
		logger: logger,
		ports:  make(map[vhciPortKey]struct{}),
	}, nil
}

func isMissingUSBDeviceError(err error) bool {
	return errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ENODEV)
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

type linuxExportHost struct {
	logger  log.ContextLogger
	matches []option.USBIPDeviceMatch

	access  sync.Mutex
	exports map[string]*linuxExport
}

func newLinuxExportHost(logger log.ContextLogger, matches []option.USBIPDeviceMatch) *linuxExportHost {
	return &linuxExportHost{
		logger:  logger,
		matches: matches,
		exports: make(map[string]*linuxExport),
	}
}

func (h *linuxExportHost) Start(ctx context.Context) error {
	return ensureKernelPath(sysUsbipHostDriver, "usbip-host", "usbip-host driver")
}

func (h *linuxExportHost) Close() error {
	h.access.Lock()
	exports := h.exports
	h.exports = make(map[string]*linuxExport)
	h.access.Unlock()
	for _, exp := range exports {
		_, statErr := os.Stat(filepath.Join(sysBusUSBDevices, exp.busid))
		restore := statErr == nil
		releaseErr := h.releaseExport(exp, restore)
		if releaseErr != nil {
			h.logger.Warn("rollback ", exp.busid, ": ", releaseErr)
		}
	}
	return nil
}

func (h *linuxExportHost) Events(ctx context.Context) (<-chan struct{}, error) {
	ch := make(chan struct{}, 1)
	go h.ueventLoop(ctx, ch)
	return ch, nil
}

func (h *linuxExportHost) ueventLoop(ctx context.Context, ch chan<- struct{}) {
	defer close(ch)
	signal := func() {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	backoff := ueventListenerBackoffInitial
	for {
		listener, err := newUEventListener()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			h.logger.Warn("open uevent listener: ", err)
			if !sleepCtx(ctx, backoff) {
				return
			}
			backoff *= 2
			if backoff > ueventListenerBackoffMax {
				backoff = ueventListenerBackoffMax
			}
			continue
		}
		backoff = ueventListenerBackoffInitial
		listenerDone := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				_ = listener.Close()
			case <-listenerDone:
			}
		}()
		signal()
		for {
			err = listener.WaitUSBEvent()
			if err != nil {
				close(listenerDone)
				_ = listener.Close()
				if ctx.Err() != nil {
					return
				}
				h.logger.Warn("read uevent: ", err)
				if !sleepCtx(ctx, backoff) {
					return
				}
				backoff *= 2
				if backoff > ueventListenerBackoffMax {
					backoff = ueventListenerBackoffMax
				}
				break
			}
			signal()
		}
	}
}

const (
	ueventListenerBackoffInitial = time.Second
	ueventListenerBackoffMax     = 30 * time.Second
)

func (h *linuxExportHost) Reconcile(ctx context.Context, isBusy func(busid string) bool) (map[string]Export, []string, error) {
	devices, err := listUSBDevices()
	if err != nil {
		return h.snapshotSelf(), nil, E.Cause(err, "enumerate usb devices")
	}
	desired := make(map[string]sysfsDevice)
	present := make(map[string]struct{}, len(devices))
	for i := range devices {
		present[devices[i].BusID] = struct{}{}
	}
	for _, m := range h.matches {
		for i := range devices {
			deviceKey := DeviceKey{
				BusID:     devices[i].BusID,
				VendorID:  devices[i].VendorID,
				ProductID: devices[i].ProductID,
				Serial:    devices[i].Serial,
			}
			if !matches(m, deviceKey) {
				continue
			}
			path := devices[i].Path
			isVHCIImport := strings.Contains(path, "vhci_hcd")
			if !isVHCIImport {
				realPath, err := filepath.EvalSymlinks(path)
				if err == nil {
					isVHCIImport = strings.Contains(realPath, "vhci_hcd")
				}
			}
			if isVHCIImport {
				h.logger.Debug("skip vhci-imported device ", devices[i].BusID, " matched by ", describeMatch(m))
				continue
			}
			if devices[i].DeviceClass == 0x09 {
				h.logger.Warn("skip hub device ", devices[i].BusID, " matched by ", describeMatch(m))
				continue
			}
			desired[devices[i].BusID] = devices[i]
		}
	}

	h.access.Lock()
	current := make(map[string]*linuxExport, len(h.exports))
	maps.Copy(current, h.exports)
	h.access.Unlock()

	for busid, device := range desired {
		if _, ok := current[busid]; ok {
			continue
		}
		exp, bindErr := h.bindOne(&device)
		if bindErr != nil {
			return h.snapshotSelf(), nil, E.Cause(bindErr, "bind ", busid)
		}
		h.access.Lock()
		h.exports[busid] = exp
		h.access.Unlock()
	}

	var released []string
	for busid, exp := range current {
		if _, ok := desired[busid]; ok {
			continue
		}
		_, restore := present[busid]
		err := h.releaseExport(exp, restore)
		if err != nil {
			h.logger.Warn("release ", busid, ": ", err)
		}
		h.access.Lock()
		delete(h.exports, busid)
		h.access.Unlock()
		released = append(released, busid)
	}

	return h.snapshotSelf(), released, nil
}

func (h *linuxExportHost) FinishImport(ctx context.Context, busid string) (bool, error) {
	err := writeSysfs(filepath.Join(sysBusUSBDevices, busid, "usbip_sockfd"), "-1")
	if err != nil && !os.IsNotExist(err) && !isMissingUSBDeviceError(err) {
		h.logger.Debug("release ", busid, " from usbip-host: ", err)
	}
	waitForUsbipStatusCleared(ctx, busid)
	return false, nil
}

func (h *linuxExportHost) snapshotSelf() map[string]Export {
	h.access.Lock()
	defer h.access.Unlock()
	out := make(map[string]Export, len(h.exports))
	for busid, exp := range h.exports {
		out[busid] = exp
	}
	return out
}

func (h *linuxExportHost) bindOne(d *sysfsDevice) (*linuxExport, error) {
	var (
		exp *linuxExport
		err error
	)
	for attempt := range 2 {
		exp, err = h.bindOneOnce(d)
		if err == nil {
			return exp, nil
		}
		if attempt > 0 || !errors.Is(err, unix.ENODEV) {
			break
		}
		h.logger.Warn("reset usbip-host after bind failure on ", d.BusID, ": ", err)
		h.access.Lock()
		active := len(h.exports) > 0
		h.access.Unlock()
		if active {
			return nil, E.Cause(E.New("active usbip-host exports are present"), "reset usbip-host after bind failure")
		}
		resetErr := reloadHostDriver()
		if resetErr != nil {
			return nil, E.Cause(resetErr, "reset usbip-host after bind failure")
		}
	}
	return nil, err
}

func (h *linuxExportHost) bindOneOnce(d *sysfsDevice) (*linuxExport, error) {
	driver, err := currentDriver(d.BusID)
	if err != nil {
		return nil, err
	}
	if driver == "usbip-host" {
		h.logger.Info("device ", d.BusID, " already bound to usbip-host; co-opting")
		return h.newExport(*d, false, ""), nil
	}
	if driver != "" {
		err = writeSysfs(filepath.Join("/sys/bus/usb/drivers", driver, "unbind"), d.BusID)
		if err != nil {
			return nil, E.Cause(err, "unbind from ", driver)
		}
	}
	matchBusIDPath := filepath.Join(sysUsbipHostDriver, "match_busid")
	err = writeSysfs(matchBusIDPath, "add "+d.BusID)
	if err != nil {
		if driver != "" {
			_ = writeSysfs(filepath.Join("/sys/bus/usb/drivers", driver, "bind"), d.BusID)
		}
		return nil, E.Cause(err, "match_busid add")
	}
	err = writeSysfs(filepath.Join(sysUsbipHostDriver, "bind"), d.BusID)
	if err != nil {
		_ = writeSysfs(matchBusIDPath, "del "+d.BusID)
		if driver != "" {
			_ = writeSysfs(filepath.Join("/sys/bus/usb/drivers", driver, "bind"), d.BusID)
		}
		return nil, E.Cause(err, "bind to usbip-host")
	}
	previousDriver := driver
	if previousDriver == "" {
		previousDriver = "(no driver)"
	}
	h.logger.Info("exported ", d.BusID, " (previously on ", previousDriver, ")")
	return h.newExport(*d, true, driver), nil
}

func (h *linuxExportHost) releaseExport(exp *linuxExport, restore bool) error {
	if !exp.managed {
		h.logger.Info("stopped tracking ", exp.busid, " on usbip-host")
		return nil
	}
	status, statusErr := readUsbipStatus(exp.busid)
	if statusErr != nil && !os.IsNotExist(statusErr) && !isMissingUSBDeviceError(statusErr) {
		return statusErr
	}
	if statusErr == nil && status == usbipStatusUsed {
		err := writeSysfs(filepath.Join(sysBusUSBDevices, exp.busid, "usbip_sockfd"), "-1")
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	err := writeSysfs(filepath.Join(sysUsbipHostDriver, "unbind"), exp.busid)
	if err != nil && !os.IsNotExist(err) && !(isMissingUSBDeviceError(err) && !restore) {
		return err
	}
	err = writeSysfs(filepath.Join(sysUsbipHostDriver, "match_busid"), "del "+exp.busid)
	if err != nil {
		return err
	}
	if !restore {
		h.logger.Info("removed export state for disappeared device ", exp.busid)
		return nil
	}
	if exp.originalDriver == "" {
		h.logger.Info("released ", exp.busid, " from usbip-host")
		return nil
	}
	err = writeSysfs(filepath.Join("/sys/bus/usb/drivers", exp.originalDriver, "bind"), exp.busid)
	if err != nil {
		return err
	}
	h.logger.Info("restored ", exp.busid, " to ", exp.originalDriver)
	return nil
}

func (h *linuxExportHost) newExport(descriptor sysfsDevice, managed bool, originalDriver string) *linuxExport {
	return &linuxExport{
		busid:          descriptor.BusID,
		descriptor:     descriptor,
		managed:        managed,
		originalDriver: originalDriver,
		logger:         h.logger,
	}
}

// linuxExport caches the bind-time descriptor because it is immutable
// post-enumeration; Snapshot only re-reads usbip_status.
type linuxExport struct {
	busid          string
	descriptor     sysfsDevice
	managed        bool
	originalDriver string
	logger         log.ContextLogger
}

func (e *linuxExport) BusID() string {
	return e.busid
}

func (e *linuxExport) Snapshot(ctx context.Context, busy bool) ExportSnapshot {
	stableID := "linux-busid:" + e.descriptor.BusID
	if e.descriptor.Serial != "" {
		stableID = fmt.Sprintf("usb:%04x:%04x:%s", e.descriptor.VendorID, e.descriptor.ProductID, e.descriptor.Serial)
	}
	status, statusErr := readUsbipStatus(e.busid)
	var state, reason string
	switch {
	case statusErr != nil:
		state = deviceStateUnavailable
		reason = statusErr.Error()
	case busy:
		status = usbipStatusUsed
		state = deviceStateBusy
		reason = linuxUSBIPStatusReason(status)
	case status == usbipStatusAvailable:
		state = deviceStateAvailable
		reason = linuxUSBIPStatusReason(status)
	case status == usbipStatusUsed:
		state = deviceStateBusy
		reason = linuxUSBIPStatusReason(status)
	default:
		state = deviceStateUnavailable
		reason = linuxUSBIPStatusReason(status)
	}
	return ExportSnapshot{
		Entry: DeviceEntry{
			Info:       e.descriptor.toProtocol(),
			Interfaces: e.descriptor.Interfaces,
			Serial:     e.descriptor.Serial,
		},
		Backend:      backendIDLinuxSysfs,
		StableID:     stableID,
		State:        state,
		StatusReason: reason,
		RawStatus:    status,
	}
}

func (e *linuxExport) LeaseCheck(ctx context.Context) (bool, string) {
	status, err := readUsbipStatus(e.busid)
	if err != nil {
		return false, err.Error()
	}
	if status != usbipStatusAvailable {
		return false, linuxUSBIPStatusReason(status)
	}
	return true, ""
}

func (e *linuxExport) DeviceInfo(ctx context.Context) (DeviceInfoTruncated, error) {
	return e.descriptor.toProtocol(), nil
}

func (e *linuxExport) NewServerDataSession(ctx context.Context, conn net.Conn) (DataSession, error) {
	handoff, err := newKernelHandoffSession(ctx, conn, e.logger, "server", e.busid)
	if err != nil {
		return nil, E.Cause(err, "prepare handoff")
	}
	mode := "direct"
	if handoff.relayConn != nil {
		mode = "relay"
	}
	e.logger.Debug("usbip server handoff ", e.busid, ": ", mode)
	err = writeSysfs(filepath.Join(sysBusUSBDevices, e.busid, "usbip_sockfd"), strconv.Itoa(int(handoff.file.Fd())))
	if err != nil {
		_ = handoff.Close()
		return nil, E.Cause(err, "hand off ", e.busid, " to kernel")
	}
	closeErr := handoff.closeKernelFD()
	if closeErr != nil {
		e.logger.Debug("close kernel fd ", e.busid, ": ", closeErr)
	}
	return handoff, nil
}

type linuxImportHost struct {
	logger      log.ContextLogger
	controllers []vhciController

	portsAccess sync.Mutex
	ports       map[vhciPortKey]struct{}
}

func (h *linuxImportHost) Start(ctx context.Context) error {
	controllers, err := discoverVHCIControllers()
	if err != nil {
		return E.Cause(err, "discover vhci controllers")
	}
	if len(controllers) == 0 {
		err = ensureKernelPath(sysVHCIControllerV0, "vhci-hcd", "vhci_hcd.0")
		if err != nil {
			return err
		}
		controllers, err = discoverVHCIControllers()
		if err != nil {
			return E.Cause(err, "discover vhci controllers")
		}
		if len(controllers) == 0 {
			return E.New("no vhci controllers present after loading vhci-hcd")
		}
	}
	h.controllers = controllers
	return nil
}

func (h *linuxImportHost) Close() error {
	return nil
}

func (h *linuxImportHost) Attach(ctx context.Context, info DeviceInfoTruncated, conn net.Conn) (AttachedSession, error) {
	handoff, err := newKernelHandoffSession(ctx, conn, h.logger, "client", info.BusIDString())
	if err != nil {
		return nil, E.Cause(err, "prepare handoff")
	}
	mode := "direct"
	if handoff.relayConn != nil {
		mode = "relay"
	}
	h.logger.Debug("usbip client handoff ", info.BusIDString(), ": ", mode)
	ctrl, port, attachErr := h.attachOnce(ctx, info, handoff)
	if attachErr != nil {
		_ = handoff.Close()
		return nil, attachErr
	}
	_ = handoff.Start()
	return &linuxClientSession{
		handoff:    handoff,
		host:       h,
		controller: ctrl,
		port:       port,
	}, nil
}

func (h *linuxImportHost) attachOnce(ctx context.Context, info DeviceInfoTruncated, handoff *kernelHandoffSession) (vhciController, int, error) {
	triedPorts := make(map[vhciPortKey]struct{})
	for {
		ctrl, port, err := vhciPickFreePort(h.controllers, info.Speed, triedPorts)
		if err != nil {
			return "", -1, err
		}
		key := vhciPortKey{controller: ctrl, port: port}
		if !h.reservePort(ctrl, port) {
			triedPorts[key] = struct{}{}
			continue
		}
		attachLine := fmt.Sprintf("%d %d %d %d", port, int(handoff.file.Fd()), info.DevID(), info.Speed)
		err = writeSysfs(filepath.Join(string(ctrl), "attach"), attachLine)
		if err != nil {
			h.releasePort(ctrl, port)
			if errors.Is(err, unix.EBUSY) {
				triedPorts[key] = struct{}{}
				continue
			}
			return "", -1, E.Cause(err, "vhci attach")
		}
		err = handoff.closeKernelFD()
		if err != nil {
			h.logger.Debug("close kernel fd ", info.BusIDString(), ": ", err)
		}
		return ctrl, port, nil
	}
}

func (h *linuxImportHost) reservePort(ctrl vhciController, port int) bool {
	key := vhciPortKey{controller: ctrl, port: port}
	h.portsAccess.Lock()
	defer h.portsAccess.Unlock()
	if _, exists := h.ports[key]; exists {
		h.logger.Debug(ctrl.name(), " port ", port, " already reserved locally")
		return false
	}
	h.logger.Debug("reserve ", ctrl.name(), " port ", port)
	h.ports[key] = struct{}{}
	return true
}

func (h *linuxImportHost) releasePort(ctrl vhciController, port int) {
	h.portsAccess.Lock()
	defer h.portsAccess.Unlock()
	h.logger.Debug("release ", ctrl.name(), " port ", port)
	delete(h.ports, vhciPortKey{controller: ctrl, port: port})
}

type linuxClientSession struct {
	handoff    *kernelHandoffSession
	host       *linuxImportHost
	controller vhciController
	port       int

	closeOnce sync.Once
	closeErr  error
}

func (s *linuxClientSession) Done() <-chan struct{} {
	return s.handoff.Done()
}

func (s *linuxClientSession) Err() error {
	return s.handoff.Err()
}

func (s *linuxClientSession) Start() error {
	return s.handoff.Start()
}

func (s *linuxClientSession) Close() error {
	s.closeOnce.Do(func() {
		detachErr := writeSysfs(filepath.Join(string(s.controller), "detach"), strconv.Itoa(s.port))
		closeErr := s.handoff.Close()
		s.host.releasePort(s.controller, s.port)
		s.closeErr = E.Errors(detachErr, closeErr)
	})
	return s.closeErr
}

func (s *linuxClientSession) Description() string {
	return fmt.Sprintf("%s port %d", s.controller.name(), s.port)
}
