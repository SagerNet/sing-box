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
	"slices"
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
		ports:  make(map[int]struct{}),
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

type linuxExportIdentity struct {
	BusNum         uint32
	DevNum         uint32
	Speed          uint32
	VendorID       uint16
	ProductID      uint16
	BCDDevice      uint16
	DeviceClass    uint8
	DeviceSubClass uint8
	DeviceProtocol uint8
	ConfigValue    uint8
	NumConfigs     uint8
	NumInterfaces  uint8
	Serial         string
	Interfaces     []DeviceInterface
}

func newLinuxExportIdentity(descriptor sysfsDevice) linuxExportIdentity {
	return linuxExportIdentity{
		BusNum:         descriptor.BusNum,
		DevNum:         descriptor.DevNum,
		Speed:          descriptor.Speed,
		VendorID:       descriptor.VendorID,
		ProductID:      descriptor.ProductID,
		BCDDevice:      descriptor.BCDDevice,
		DeviceClass:    descriptor.DeviceClass,
		DeviceSubClass: descriptor.DeviceSubClass,
		DeviceProtocol: descriptor.DeviceProtocol,
		ConfigValue:    descriptor.ConfigValue,
		NumConfigs:     descriptor.NumConfigs,
		NumInterfaces:  descriptor.NumInterfaces,
		Serial:         descriptor.Serial,
		Interfaces:     slices.Clone(descriptor.Interfaces),
	}
}

func (i linuxExportIdentity) Equal(other linuxExportIdentity) bool {
	if i.BusNum != other.BusNum ||
		i.DevNum != other.DevNum ||
		i.Speed != other.Speed ||
		i.VendorID != other.VendorID ||
		i.ProductID != other.ProductID ||
		i.BCDDevice != other.BCDDevice ||
		i.DeviceClass != other.DeviceClass ||
		i.DeviceSubClass != other.DeviceSubClass ||
		i.DeviceProtocol != other.DeviceProtocol ||
		i.ConfigValue != other.ConfigValue ||
		i.NumConfigs != other.NumConfigs ||
		i.NumInterfaces != other.NumInterfaces ||
		i.Serial != other.Serial ||
		len(i.Interfaces) != len(other.Interfaces) {
		return false
	}
	for index := range i.Interfaces {
		if i.Interfaces[index] != other.Interfaces[index] {
			return false
		}
	}
	return true
}

func (i linuxExportIdentity) LeaseIdentity() ExportLeaseIdentity {
	var builder strings.Builder
	fmt.Fprintf(&builder, "linux:%d:%d:%d:%04x:%04x:%04x:%02x:%02x:%02x:%02x:%02x:%02x:%s",
		i.BusNum,
		i.DevNum,
		i.Speed,
		i.VendorID,
		i.ProductID,
		i.BCDDevice,
		i.DeviceClass,
		i.DeviceSubClass,
		i.DeviceProtocol,
		i.ConfigValue,
		i.NumConfigs,
		i.NumInterfaces,
		i.Serial,
	)
	for _, iface := range i.Interfaces {
		fmt.Fprintf(&builder, "|%02x.%02x.%02x", iface.BInterfaceClass, iface.BInterfaceSubClass, iface.BInterfaceProtocol)
	}
	return ExportLeaseIdentity(builder.String())
}

type linuxExportHost struct {
	logger  log.ContextLogger
	matches []option.USBIPDeviceMatch

	access  sync.Mutex
	exports map[string]*linuxExport
}

type linuxReconcilePlan struct {
	toRelease []*linuxExport
	toStale   []string
	toBind    map[string]sysfsDevice
	released  []string
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
		releaseErr := h.releaseExport(exp, exp.shouldRestoreCurrentDevice())
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

func classifyLinuxReconcile(current map[string]*linuxExport, desired map[string]sysfsDevice, isBusy func(busid string) bool) linuxReconcilePlan {
	remainingDesired := maps.Clone(desired)
	plan := linuxReconcilePlan{
		toBind: make(map[string]sysfsDevice),
	}
	for busid, exp := range current {
		device, wanted := remainingDesired[busid]
		busy := isBusy(busid)
		identityMatches := wanted && exp.identity.Equal(newLinuxExportIdentity(device))
		switch {
		case exp.stale:
			if busy {
				delete(remainingDesired, busid)
				continue
			}
			plan.toRelease = append(plan.toRelease, exp)
			plan.released = append(plan.released, busid)
		case identityMatches:
			delete(remainingDesired, busid)
		case busy:
			plan.toStale = append(plan.toStale, busid)
			delete(remainingDesired, busid)
		default:
			plan.toRelease = append(plan.toRelease, exp)
			plan.released = append(plan.released, busid)
		}
	}
	for busid, device := range remainingDesired {
		if isBusy(busid) {
			continue
		}
		plan.toBind[busid] = device
	}
	return plan
}

func (h *linuxExportHost) Reconcile(ctx context.Context, isBusy func(busid string) bool) (map[string]Export, []string, error) {
	devices, err := listUSBDevices()
	if err != nil {
		return h.snapshotSelf(), nil, E.Cause(err, "enumerate usb devices")
	}
	desired := make(map[string]sysfsDevice)
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

	plan := classifyLinuxReconcile(current, desired, isBusy)
	toAdd := make([]*linuxExport, 0, len(plan.toBind))
	for _, exp := range plan.toRelease {
		err := h.releaseExport(exp, false)
		if err != nil {
			h.logger.Warn("release ", exp.busid, ": ", err)
		}
	}

	for busid, device := range plan.toBind {
		exp, bindErr := h.bindOne(&device)
		if bindErr != nil {
			return h.snapshotSelf(), nil, E.Cause(bindErr, "bind ", busid)
		}
		toAdd = append(toAdd, exp)
	}

	h.access.Lock()
	for _, busid := range plan.toStale {
		exp, ok := h.exports[busid]
		if ok {
			exp.stale = true
		}
	}
	for _, exp := range plan.toRelease {
		currentExport, ok := h.exports[exp.busid]
		if ok && currentExport == exp {
			delete(h.exports, exp.busid)
		}
	}
	for _, exp := range toAdd {
		h.exports[exp.busid] = exp
	}
	h.access.Unlock()

	return h.snapshotSelf(), plan.released, nil
}

func (h *linuxExportHost) FinishImport(ctx context.Context, busid string) (bool, error) {
	err := writeSysfs(filepath.Join(sysBusUSBDevices, busid, "usbip_sockfd"), "-1")
	if err != nil && !os.IsNotExist(err) && !isMissingUSBDeviceError(err) {
		h.logger.Debug("release ", busid, " from usbip-host: ", err)
	}
	waitForUsbipStatusCleared(ctx, busid)
	h.access.Lock()
	exp, ok := h.exports[busid]
	h.access.Unlock()
	if !ok || !exp.stale {
		return false, nil
	}
	releaseErr := h.releaseExport(exp, false)
	h.access.Lock()
	current, stillPresent := h.exports[busid]
	if stillPresent && current == exp {
		delete(h.exports, busid)
	}
	h.access.Unlock()
	if releaseErr != nil {
		h.logger.Warn("release stale ", busid, ": ", releaseErr)
	}
	return true, E.Errors(err, releaseErr)
}

func (h *linuxExportHost) snapshotSelf() map[string]Export {
	h.access.Lock()
	defer h.access.Unlock()
	out := make(map[string]Export, len(h.exports))
	for busid, exp := range h.exports {
		if exp.stale {
			continue
		}
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
		h.logger.Info("removed export state for ", exp.busid)
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
		identity:       newLinuxExportIdentity(descriptor),
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
	identity       linuxExportIdentity
	managed        bool
	originalDriver string
	logger         log.ContextLogger
	stale          bool
}

func (e *linuxExport) BusID() string {
	return e.busid
}

func (e *linuxExport) LeaseIdentity() ExportLeaseIdentity {
	return e.identity.LeaseIdentity()
}

func (e *linuxExport) Snapshot(ctx context.Context, busy bool) ExportSnapshot {
	stableID := "linux-busid:" + e.descriptor.BusID
	if e.descriptor.Serial != "" {
		stableID = fmt.Sprintf("usb:%04x:%04x:%s", e.descriptor.VendorID, e.descriptor.ProductID, e.descriptor.Serial)
	}
	if e.stale {
		return ExportSnapshot{
			Entry: DeviceEntry{
				Info:       e.descriptor.toProtocol(),
				Interfaces: e.descriptor.Interfaces,
				Serial:     e.descriptor.Serial,
			},
			Backend:      backendIDLinuxSysfs,
			StableID:     stableID,
			State:        deviceStateUnavailable,
			StatusReason: "device replaced",
		}
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
	if e.stale {
		return false, "device replaced"
	}
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
	if e.stale {
		return nil, E.New("linux export ", e.busid, " is stale")
	}
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

func (e *linuxExport) shouldRestoreCurrentDevice() bool {
	if e.originalDriver == "" || e.stale {
		return false
	}
	descriptor, err := readSysfsDevice(e.busid, filepath.Join(sysBusUSBDevices, e.busid))
	if err != nil {
		return false
	}
	return e.identity.Equal(newLinuxExportIdentity(descriptor))
}

type linuxImportHost struct {
	logger log.ContextLogger

	portsAccess sync.Mutex
	ports       map[int]struct{}
}

func (h *linuxImportHost) Start(ctx context.Context) error {
	return ensureKernelPath(sysVHCIControllerV0, "vhci-hcd", "vhci_hcd.0")
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
	port, secondary, attachErr := h.attachOnce(ctx, info, handoff)
	if attachErr != nil {
		_ = handoff.Close()
		return nil, attachErr
	}
	_ = handoff.Start()
	return &linuxClientSession{
		handoff:   handoff,
		host:      h,
		port:      port,
		secondary: secondary,
	}, nil
}

func (h *linuxImportHost) attachOnce(ctx context.Context, info DeviceInfoTruncated, handoff *kernelHandoffSession) (int, int, error) {
	triedPorts := make(map[int]struct{})
	for {
		port, err := vhciPickFreePort(info.Speed, triedPorts)
		if err != nil {
			return -1, 0, err
		}
		if !h.reservePort(port) {
			triedPorts[port] = struct{}{}
			continue
		}
		attachLine := fmt.Sprintf("%d %d %d %d", port, int(handoff.file.Fd()), info.DevID(), info.Speed)
		err = writeSysfs(filepath.Join(sysVHCIControllerV0, "attach"), attachLine)
		if err != nil {
			h.releasePort(port)
			if errors.Is(err, unix.EBUSY) {
				triedPorts[port] = struct{}{}
				continue
			}
			return -1, 0, E.Cause(err, "vhci attach")
		}
		err = handoff.closeKernelFD()
		if err != nil {
			h.logger.Debug("close kernel fd ", info.BusIDString(), ": ", err)
		}
		return port, lookupSecondaryForPort(port), nil
	}
}

func lookupSecondaryForPort(port int) int {
	records, err := readPrimaryVHCIStatus()
	if err != nil {
		return 0
	}
	for _, record := range records {
		if record.port == port {
			return record.secondary
		}
	}
	return 0
}

func (h *linuxImportHost) reservePort(port int) bool {
	h.portsAccess.Lock()
	defer h.portsAccess.Unlock()
	_, exists := h.ports[port]
	if exists {
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
	handoff   *kernelHandoffSession
	host      *linuxImportHost
	port      int
	secondary int

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
		detachErr := writeSysfs(filepath.Join(sysVHCIControllerV0, "detach"), strconv.Itoa(s.port))
		closeErr := s.handoff.Close()
		s.host.releasePort(s.port)
		s.closeErr = E.Errors(detachErr, closeErr)
	})
	return s.closeErr
}

func (s *linuxClientSession) Description() string {
	if s.secondary == 0 {
		return fmt.Sprintf("vhci_hcd.0 port %d", s.port)
	}
	return fmt.Sprintf("vhci_hcd.0 (controller %d) port %d", s.secondary, s.port)
}
