//go:build linux

package usbip

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
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
	return newLinuxImportHost(logger), nil
}

func sysBusDevicePath(busid string) string {
	return sysBusUSBDevices + "/" + busid
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
	return ensureHostDriver()
}

func (h *linuxExportHost) Close() error {
	h.access.Lock()
	exports := h.exports
	h.exports = make(map[string]*linuxExport)
	h.access.Unlock()
	for _, exp := range exports {
		_, statErr := os.Stat(sysBusDevicePath(exp.busid))
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
			backoff = nextUEventListenerBackoff(backoff)
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
				backoff = nextUEventListenerBackoff(backoff)
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

func nextUEventListenerBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next > ueventListenerBackoffMax {
		return ueventListenerBackoffMax
	}
	return next
}

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
			if !matches(m, devices[i].key()) {
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
	for busid, exp := range h.exports {
		current[busid] = exp
	}
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
	err := writeUsbipSockfd(busid, -1)
	if err != nil && !os.IsNotExist(err) && !isMissingUSBDeviceError(err) {
		h.logger.Debug("release ", busid, " from usbip-host: ", err)
	}
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
	for attempt := 0; attempt < 2; attempt++ {
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
		err = unbindFromDriver(d.BusID, driver)
		if err != nil {
			return nil, E.Cause(err, "unbind from ", driver)
		}
	}
	err = hostMatchBusID(d.BusID, true)
	if err != nil {
		if driver != "" {
			_ = bindToDriver(d.BusID, driver)
		}
		return nil, E.Cause(err, "match_busid add")
	}
	err = hostBind(d.BusID)
	if err != nil {
		_ = hostMatchBusID(d.BusID, false)
		if driver != "" {
			_ = bindToDriver(d.BusID, driver)
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
		err := writeUsbipSockfd(exp.busid, -1)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	err := hostUnbind(exp.busid)
	if err != nil && !os.IsNotExist(err) && !(isMissingUSBDeviceError(err) && !restore) {
		return err
	}
	err = hostMatchBusID(exp.busid, false)
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
	err = bindToDriver(exp.busid, exp.originalDriver)
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
		Entry:        e.descriptor.toDeviceEntry(),
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
	handoff, err := newKernelHandoffSession(conn)
	if err != nil {
		return nil, E.Cause(err, "prepare handoff")
	}
	e.logger.Debug("usbip server handoff ", e.busid, ": ", handoff.mode())
	err = writeUsbipSockfd(e.busid, int(handoff.kernelFD()))
	if err != nil {
		_ = handoff.Close()
		return nil, E.Cause(err, "hand off ", e.busid, " to kernel")
	}
	closeErr := handoff.closeKernelFD()
	if closeErr != nil {
		e.logger.Debug("close kernel fd ", e.busid, ": ", closeErr)
	}
	handoff.Start(ctx, e.logger, "server", e.busid)
	return handoff, nil
}

type linuxImportHost struct {
	logger log.ContextLogger

	portsAccess sync.Mutex
	ports       map[int]struct{}
}

func newLinuxImportHost(logger log.ContextLogger) *linuxImportHost {
	return &linuxImportHost{
		logger: logger,
		ports:  make(map[int]struct{}),
	}
}

func (h *linuxImportHost) Start(ctx context.Context) error {
	return ensureVHCI()
}

func (h *linuxImportHost) Close() error {
	return nil
}

func (h *linuxImportHost) Attach(ctx context.Context, info DeviceInfoTruncated, conn net.Conn) (AttachedSession, error) {
	handoff, err := newKernelHandoffSession(conn)
	if err != nil {
		return nil, E.Cause(err, "prepare handoff")
	}
	h.logger.Debug("usbip client handoff ", info.BusIDString(), ": ", handoff.mode())
	port, attachErr := h.attachOnce(ctx, info, handoff)
	if attachErr != nil {
		_ = handoff.Close()
		return nil, attachErr
	}
	handoff.Start(ctx, h.logger, "client", info.BusIDString())
	return &linuxClientSession{
		handoff: handoff,
		host:    h,
		port:    port,
	}, nil
}

func (h *linuxImportHost) attachOnce(ctx context.Context, info DeviceInfoTruncated, handoff *kernelHandoffSession) (int, error) {
	triedPorts := make(map[int]struct{})
	for {
		port, err := vhciPickFreePort(info.Speed, triedPorts)
		if err != nil {
			return -1, err
		}
		if !h.reservePort(port) {
			triedPorts[port] = struct{}{}
			continue
		}
		err = vhciAttach(port, handoff.kernelFD(), info.DevID(), info.Speed)
		if err != nil {
			h.releasePort(port)
			if errors.Is(err, unix.EBUSY) {
				triedPorts[port] = struct{}{}
				continue
			}
			return -1, E.Cause(err, "vhci attach")
		}
		err = handoff.closeKernelFD()
		if err != nil {
			h.logger.Debug("close kernel fd ", info.BusIDString(), ": ", err)
		}
		return port, nil
	}
}

func (h *linuxImportHost) reservePort(port int) bool {
	h.portsAccess.Lock()
	defer h.portsAccess.Unlock()
	if _, exists := h.ports[port]; exists {
		h.logger.Debug("vhci port ", port, " already reserved locally")
		return false
	}
	h.logger.Debug("reserve vhci port ", port)
	h.ports[port] = struct{}{}
	return true
}

func (h *linuxImportHost) releasePort(port int) {
	h.portsAccess.Lock()
	defer h.portsAccess.Unlock()
	h.logger.Debug("release vhci port ", port)
	delete(h.ports, port)
}

type linuxClientSession struct {
	handoff *kernelHandoffSession
	host    *linuxImportHost
	port    int

	closeOnce sync.Once
	closeErr  error
}

func (s *linuxClientSession) Done() <-chan struct{} {
	return s.handoff.Done()
}

func (s *linuxClientSession) Err() error {
	return s.handoff.Err()
}

func (s *linuxClientSession) Close() error {
	s.closeOnce.Do(func() {
		detachErr := vhciDetach(s.port)
		closeErr := s.handoff.Close()
		s.host.releasePort(s.port)
		s.closeErr = E.Errors(detachErr, closeErr)
	})
	return s.closeErr
}

func (s *linuxClientSession) Description() string {
	return fmt.Sprintf("vhci port %d", s.port)
}
