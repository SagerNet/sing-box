//go:build linux

package usbip

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/unix"
)

// linuxExportHost adapts the Linux sysfs / usbip-host pipeline to the
// ExportHost interface. It owns the platform-specific bind/unbind dance
// and the per-export resource state; the service owns busy tracking
// and broadcast lifecycle.
type linuxExportHost struct {
	logger  log.ContextLogger
	matches []option.USBIPDeviceMatch
	ops     usbipOps

	access  sync.Mutex
	exports map[string]*linuxExport
}

func newLinuxExportHost(logger log.ContextLogger, matches []option.USBIPDeviceMatch, ops usbipOps) *linuxExportHost {
	return &linuxExportHost{
		logger:  logger,
		matches: matches,
		ops:     ops,
		exports: make(map[string]*linuxExport),
	}
}

func (h *linuxExportHost) Start(ctx context.Context) error {
	return h.ops.ensureHostDriver()
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
		listener, err := h.ops.newUEventListener()
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

func (h *linuxExportHost) Reconcile(ctx context.Context, isBusy func(busid string) bool) (map[string]Export, []string, bool, error) {
	devices, err := h.ops.listUSBDevices()
	if err != nil {
		return h.snapshotSelf(), nil, false, E.Cause(err, "enumerate usb devices")
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
			if isVHCIImportedDevice(devices[i].Path) {
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

	changed := false
	for busid, device := range desired {
		if _, ok := current[busid]; ok {
			continue
		}
		exp, bindErr := h.bindOne(&device)
		if bindErr != nil {
			return h.snapshotSelf(), nil, changed, E.Cause(bindErr, "bind ", busid)
		}
		h.access.Lock()
		h.exports[busid] = exp
		h.access.Unlock()
		changed = true
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
		changed = true
	}

	return h.snapshotSelf(), released, changed, nil
}

func (h *linuxExportHost) FinishImport(ctx context.Context, busid string) (bool, error) {
	err := h.ops.writeUsbipSockfd(busid, -1)
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
		if resetErr := h.resetHostDriverForBindRetry(); resetErr != nil {
			return nil, E.Cause(resetErr, "reset usbip-host after bind failure")
		}
	}
	return nil, err
}

func (h *linuxExportHost) bindOneOnce(d *sysfsDevice) (*linuxExport, error) {
	driver, err := h.ops.currentDriver(d.BusID)
	if err != nil {
		return nil, err
	}
	if driver == "usbip-host" {
		h.logger.Info("device ", d.BusID, " already bound to usbip-host; co-opting")
		return h.newExport(*d, false, ""), nil
	}
	if driver != "" {
		err = h.ops.unbindFromDriver(d.BusID, driver)
		if err != nil {
			return nil, E.Cause(err, "unbind from ", driver)
		}
	}
	err = h.ops.hostMatchBusID(d.BusID, true)
	if err != nil {
		if driver != "" {
			_ = h.ops.bindToDriver(d.BusID, driver)
		}
		return nil, E.Cause(err, "match_busid add")
	}
	err = h.ops.hostBind(d.BusID)
	if err != nil {
		_ = h.ops.hostMatchBusID(d.BusID, false)
		if driver != "" {
			_ = h.ops.bindToDriver(d.BusID, driver)
		}
		return nil, E.Cause(err, "bind to usbip-host")
	}
	h.logger.Info("exported ", d.BusID, " (previously on ", driverOrNone(driver), ")")
	return h.newExport(*d, true, driver), nil
}

func (h *linuxExportHost) resetHostDriverForBindRetry() error {
	h.access.Lock()
	active := len(h.exports) > 0
	h.access.Unlock()
	if active {
		return E.New("active usbip-host exports are present")
	}
	return h.ops.reloadHostDriver()
}

func (h *linuxExportHost) releaseExport(exp *linuxExport, restore bool) error {
	if !exp.managed {
		h.logger.Info("stopped tracking ", exp.busid, " on usbip-host")
		return nil
	}
	status, statusErr := h.ops.readUsbipStatus(exp.busid)
	if statusErr != nil && !os.IsNotExist(statusErr) && !isMissingUSBDeviceError(statusErr) {
		return statusErr
	}
	if statusErr == nil && status == usbipStatusUsed {
		err := h.ops.writeUsbipSockfd(exp.busid, -1)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	err := h.ops.hostUnbind(exp.busid)
	if err != nil && !os.IsNotExist(err) && !(isMissingUSBDeviceError(err) && !restore) {
		return err
	}
	err = h.ops.hostMatchBusID(exp.busid, false)
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
	err = h.ops.bindToDriver(exp.busid, exp.originalDriver)
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
		ops:            h.ops,
		logger:         h.logger,
	}
}

// linuxExport caches the device descriptor read at bind time. The
// descriptor is immutable post-enumeration, so Snapshot only re-reads
// usbip_status to track lease/import state changes.
type linuxExport struct {
	busid          string
	descriptor     sysfsDevice
	managed        bool
	originalDriver string
	ops            usbipOps
	logger         log.ContextLogger
}

func (e *linuxExport) BusID() string {
	return e.busid
}

func (e *linuxExport) Snapshot(ctx context.Context, busy bool) ExportSnapshot {
	backend := backendIDLinuxSysfs
	stableID := linuxStableID(e.descriptor)
	status, statusErr := e.ops.readUsbipStatus(e.busid)
	state := linuxUSBIPStatusState(status)
	reason := linuxUSBIPStatusReason(status)
	if statusErr != nil {
		state = deviceStateUnavailable
		reason = statusErr.Error()
	} else if busy {
		status = usbipStatusUsed
		state = deviceStateBusy
		reason = linuxUSBIPStatusReason(status)
	}
	return ExportSnapshot{
		Entry:        e.descriptor.toDeviceEntry(),
		Backend:      backend,
		StableID:     stableID,
		State:        state,
		StatusReason: reason,
		RawStatus:    status,
	}
}

func (e *linuxExport) LeaseCheck(ctx context.Context) (bool, string) {
	status, err := e.ops.readUsbipStatus(e.busid)
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
	err = e.ops.writeUsbipSockfd(e.busid, int(handoff.kernelFD()))
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

// linuxImportHost adapts the Linux vhci_hcd attach pipeline to the
// ImportHost interface. It owns per-port reservation and runs the
// kernel handoff after vhci attach succeeds.
type linuxImportHost struct {
	logger log.ContextLogger
	ops    usbipOps

	portsAccess sync.Mutex
	ports       map[int]struct{}
}

func newLinuxImportHost(logger log.ContextLogger, ops usbipOps) *linuxImportHost {
	return &linuxImportHost{
		logger: logger,
		ops:    ops,
		ports:  make(map[int]struct{}),
	}
}

func (h *linuxImportHost) Start(ctx context.Context) error {
	return h.ops.ensureVHCI()
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
		port, err := h.ops.vhciPickFreePort(info.Speed, triedPorts)
		if err != nil {
			return -1, err
		}
		if !h.reservePort(port) {
			triedPorts[port] = struct{}{}
			continue
		}
		err = h.ops.vhciAttach(port, handoff.kernelFD(), info.DevID(), info.Speed)
		if err != nil {
			h.trackPort(port, false)
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

func (h *linuxImportHost) trackPort(port int, add bool) {
	h.portsAccess.Lock()
	defer h.portsAccess.Unlock()
	if add {
		h.logger.Debug("reserve vhci port ", port)
		h.ports[port] = struct{}{}
	} else {
		h.logger.Debug("release vhci port ", port)
		delete(h.ports, port)
	}
}

// linuxClientSession wraps kernelHandoffSession with vhci-port cleanup
// at Close time.
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
		detachErr := s.host.ops.vhciDetach(s.port)
		closeErr := s.handoff.Close()
		s.host.trackPort(s.port, false)
		s.closeErr = E.Errors(detachErr, closeErr)
	})
	return s.closeErr
}

func (s *linuxClientSession) Description() string {
	return fmt.Sprintf("vhci port %d", s.port)
}
