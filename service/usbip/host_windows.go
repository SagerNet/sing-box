//go:build windows

package usbip

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sagernet/sing-box/common/vboxusb"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func newPlatformExportHost(ctx context.Context, logger log.ContextLogger, matches []option.USBIPDeviceMatch) (ExportHost, error) {
	return newWindowsExportHost(ctx, logger, matches), nil
}

// newPlatformImportHost: usbip-client is not yet implemented on
// Windows. A virtual host controller driver is required to drive
// imported devices into the local USB stack — that is a separate
// effort tracked outside this PR.
func newPlatformImportHost(_ log.ContextLogger) (ImportHost, error) {
	return nil, E.New("usbip-client service is not yet implemented on Windows")
}

// windowsExportHost manages one VBoxUSBMon handle and one filter id
// per actively-matched device. Reconcile is called by ServerService
// on a timer (Events delivers ticks because CM_Register_Notification
// is not yet wired up — see vboxusb/pnp_windows.go).
type windowsExportHost struct {
	logger  log.ContextLogger
	matches []option.USBIPDeviceMatch

	runCtx    context.Context
	runCancel context.CancelFunc

	access  sync.Mutex
	monitor *vboxusb.Monitor
	exports map[string]*windowsExport
	filters map[string]uint64 // busid -> VBoxUSBMon filter id
}

func newWindowsExportHost(ctx context.Context, logger log.ContextLogger, matches []option.USBIPDeviceMatch) *windowsExportHost {
	runCtx, runCancel := context.WithCancel(ctx)
	return &windowsExportHost{
		logger:    logger,
		matches:   matches,
		runCtx:    runCtx,
		runCancel: runCancel,
		exports:   make(map[string]*windowsExport),
		filters:   make(map[string]uint64),
	}
}

func (h *windowsExportHost) Start() error {
	err := vboxusb.EnableLoadDriverPrivilege()
	if err != nil {
		return E.Cause(err, "windows usbip: enable SeLoadDriverPrivilege")
	}
	err = vboxusb.EnsureDrivers()
	if err != nil {
		return E.Cause(err, "windows usbip: install VBoxUSB drivers")
	}
	monitor, err := vboxusb.OpenMonitor()
	if err != nil {
		return E.Cause(err, "windows usbip: open VBoxUSBMon")
	}
	major, minor, err := monitor.GetVersion()
	if err != nil {
		_ = monitor.Close()
		return E.Cause(err, "windows usbip: monitor GET_VERSION")
	}
	if major != vboxusb.DriverMajorVersion {
		_ = monitor.Close()
		return E.New("windows usbip: VBoxUSBMon major version ", major, " (need ", vboxusb.DriverMajorVersion, ")")
	}
	h.logger.Info("VBoxUSBMon ", major, ".", minor, " ready")
	h.access.Lock()
	h.monitor = monitor
	h.access.Unlock()
	return nil
}

func (h *windowsExportHost) Close() error {
	h.runCancel()
	h.access.Lock()
	monitor := h.monitor
	h.monitor = nil
	filters := h.filters
	h.filters = make(map[string]uint64)
	exports := h.exports
	h.exports = make(map[string]*windowsExport)
	h.access.Unlock()
	if monitor != nil {
		for busid, id := range filters {
			err := monitor.RemoveFilter(id)
			if err != nil {
				h.logger.Debug("remove filter for ", busid, ": ", err)
			}
		}
		_ = monitor.Close()
	}
	for _, exp := range exports {
		if exp.device != nil {
			_ = exp.device.Close()
		}
	}
	return nil
}

// Events returns a channel that the server polls for topology changes.
// On Windows, CM_Register_Notification wiring is a Phase B follow-up;
// for now we tick on a fixed interval so Reconcile runs periodically.
// Once the notification hook lands, this becomes edge-triggered.
func (h *windowsExportHost) Events() (<-chan struct{}, error) {
	ch := make(chan struct{}, 1)
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-h.runCtx.Done():
				close(ch)
				return
			case <-ticker.C:
				select {
				case ch <- struct{}{}:
				default:
				}
			}
		}
	}()
	return ch, nil
}

func (h *windowsExportHost) Reconcile(isReserved func(busid string) bool) (map[string]Export, []string, error) {
	devices, err := vboxusb.EnumerateUSBDevices()
	if err != nil {
		return h.snapshotSelf(), nil, E.Cause(err, "windows usbip: enumerate USB devices")
	}
	keys := make([]DeviceKey, 0, len(devices))
	for _, d := range devices {
		keys = append(keys, DeviceKey{
			BusID:     d.BusID,
			VendorID:  d.VendorID,
			ProductID: d.ProductID,
		})
	}
	desired := make(map[string]vboxusb.USBDeviceInfo)
	for _, idx := range SelectMatches(h.matches, keys) {
		info := devices[idx]
		if info.DeviceClass == 0x09 {
			h.logger.Warn("skip hub device ", info.BusID)
			continue
		}
		desired[info.BusID] = info
	}

	h.access.Lock()
	monitor := h.monitor
	current := make(map[string]*windowsExport, len(h.exports))
	for k, v := range h.exports {
		current[k] = v
	}
	h.access.Unlock()

	var released []string
	for busid, info := range desired {
		if _, ok := current[busid]; ok {
			continue
		}
		exp := newWindowsExport(info, h.logger)
		h.installFilterLocked(monitor, busid, info)
		current[busid] = exp
		h.logger.Info("matched ", busid, " (vid=", fmt.Sprintf("0x%04x", info.VendorID), " pid=", fmt.Sprintf("0x%04x", info.ProductID), ") — capture pending PnP arrival")
	}
	for busid := range current {
		if _, ok := desired[busid]; ok {
			continue
		}
		if isReserved(busid) {
			continue
		}
		h.removeFilterLocked(monitor, busid)
		delete(current, busid)
		released = append(released, busid)
		h.logger.Info("released ", busid, " (no longer matches)")
	}

	h.access.Lock()
	h.exports = current
	h.access.Unlock()

	return h.snapshotSelf(), released, nil
}

func (h *windowsExportHost) FinishImport(busid string) (bool, error) {
	h.access.Lock()
	exp, ok := h.exports[busid]
	h.access.Unlock()
	if !ok {
		return false, nil
	}
	if exp.device != nil {
		_ = exp.device.Close()
		exp.device = nil
	}
	// TODO(phase B follow-up): once vboxusb.RestartDevice is implemented,
	// call it here (or CycleHubPort) so Windows re-binds the device's
	// natural function driver. Until then, the device stays in VBoxUSB's
	// captured state until physical unplug.
	return false, nil
}

func (h *windowsExportHost) snapshotSelf() map[string]Export {
	h.access.Lock()
	defer h.access.Unlock()
	out := make(map[string]Export, len(h.exports))
	for k, v := range h.exports {
		out[k] = v
	}
	return out
}

// installFilterLocked is called from Reconcile under the assumption
// the caller already holds (or does not need) the lock for this
// h.filters mutation — but we re-lock briefly to keep the mutation
// safe.
func (h *windowsExportHost) installFilterLocked(monitor *vboxusb.Monitor, busid string, info vboxusb.USBDeviceInfo) {
	if monitor == nil {
		return
	}
	vendor := info.VendorID
	product := info.ProductID
	filterID, err := monitor.AddFilter(vboxusb.Filter{
		VendorID:  &vendor,
		ProductID: &product,
	})
	if err != nil {
		h.logger.Warn("ADD_FILTER for ", busid, ": ", err)
		return
	}
	h.access.Lock()
	h.filters[busid] = filterID
	h.access.Unlock()
}

func (h *windowsExportHost) removeFilterLocked(monitor *vboxusb.Monitor, busid string) {
	if monitor == nil {
		return
	}
	h.access.Lock()
	id, ok := h.filters[busid]
	delete(h.filters, busid)
	h.access.Unlock()
	if !ok {
		return
	}
	err := monitor.RemoveFilter(id)
	if err != nil {
		h.logger.Debug("remove filter for ", busid, ": ", err)
	}
}

// windowsExport carries the per-device state needed to satisfy the
// Export interface and to drive the claim sequence in NewServerDataSession.
type windowsExport struct {
	info   vboxusb.USBDeviceInfo
	entry  DeviceEntry
	logger log.ContextLogger
	device *vboxusb.Device
}

func newWindowsExport(info vboxusb.USBDeviceInfo, logger log.ContextLogger) *windowsExport {
	entry := DeviceEntry{
		Info: DeviceInfoTruncated{
			BusNum:             info.BusNumber,
			DevNum:             info.Address,
			IDVendor:           info.VendorID,
			IDProduct:          info.ProductID,
			BCDDevice:          info.Revision,
			BDeviceClass:       info.DeviceClass,
			BNumConfigurations: 1,
		},
	}
	copy(entry.Info.Path[:], "/sys/bus/usb/devices/"+info.BusID)
	copy(entry.Info.BusID[:], info.BusID)
	return &windowsExport{info: info, entry: entry, logger: logger}
}

func (e *windowsExport) BusID() string {
	return e.info.BusID
}

func (e *windowsExport) Snapshot(busy bool) ExportSnapshot {
	state := deviceStateAvailable
	if busy {
		state = deviceStateBusy
	}
	return ExportSnapshot{
		Entry:    e.entry,
		Backend:  backendIDWindowsVBoxUSB,
		StableID: "windows-instance:" + e.info.InstanceID,
		State:    state,
	}
}

func (e *windowsExport) DeviceInfo() (DeviceInfoTruncated, error) {
	return e.entry.Info, nil
}

// NewServerDataSession executes the VBoxUSB claim sequence:
//
//  1. Wait for the VBoxUSB-bound device interface to appear under
//     GUID_CLASS_VBOXUSB. This only resolves if VBoxUSBMon's filter has
//     already triggered (next PnP arrival after AddFilter); for devices
//     already plugged in when sing-box started, the user must physically
//     unplug/replug, or RestartDevice must complete (Phase B follow-up).
//  2. Open the device handle and verify driver version.
//  3. USB_CLAIM_DEVICE to take exclusive ownership.
//  4. Wrap as vboxusbEngine and hand to userspaceURBSession.
func (e *windowsExport) NewServerDataSession(ctx context.Context, conn net.Conn) (DataSession, error) {
	path, err := vboxusb.WaitForVBoxUSBInterface(e.info.InstanceID, 10*time.Second)
	if err != nil {
		return nil, E.Cause(err, "windows usbip: locate VBoxUSB interface")
	}
	device, err := vboxusb.OpenDevice(path)
	if err != nil {
		return nil, E.Cause(err, "windows usbip: open VBoxUSB device")
	}
	major, minor, err := device.GetVersion()
	if err != nil {
		_ = device.Close()
		return nil, E.Cause(err, "windows usbip: device GET_VERSION")
	}
	if major != vboxusb.DriverMajorVersion {
		_ = device.Close()
		return nil, E.New("windows usbip: VBoxUSB major version ", major, ".", minor, " (need ", vboxusb.DriverMajorVersion, ")")
	}
	claimed, err := device.Claim()
	if err != nil {
		_ = device.Close()
		return nil, E.Cause(err, "windows usbip: claim device")
	}
	if !claimed {
		_ = device.Close()
		return nil, E.New("windows usbip: device ", e.info.BusID, " is already claimed by another handle")
	}
	e.device = device
	return newUserspaceURBSession(ctx, e.logger, conn, newVBoxUSBEngine(device)), nil
}
