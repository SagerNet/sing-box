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

func newPlatformImportHost(logger log.ContextLogger) (ImportHost, error) {
	return &windowsImportHost{logger: logger}, nil
}

// windowsExportAbsenceGrace covers the window in which a device under
// capture/release restart is removed from the PnP tree and therefore
// missing from enumeration. Exports seen more recently than this are
// not dropped just because the device is momentarily absent.
const windowsExportAbsenceGrace = 10 * time.Second

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
	h.monitor = monitor
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
	for busid, exp := range exports {
		device := exp.takeDevice()
		if device != nil {
			_ = device.Close()
		}
		instanceID, captured := exp.lastState()
		var restart *vboxusb.DeviceRestart
		if captured {
			var err error
			restart, err = vboxusb.BeginDeviceRestart(instanceID, exp.info.Address)
			if err != nil {
				h.logger.Warn("restart ", busid, " for release: ", err)
			}
		}
		if monitor != nil {
			if id, ok := filters[busid]; ok {
				delete(filters, busid)
				err := monitor.RemoveFilter(id)
				if err != nil {
					h.logger.Debug("remove filter for ", busid, ": ", err)
				}
			}
		}
		if restart != nil {
			restart.Finish()
		}
	}
	if monitor != nil {
		for busid, id := range filters {
			err := monitor.RemoveFilter(id)
			if err != nil {
				h.logger.Debug("remove filter for ", busid, ": ", err)
			}
		}
		_ = monitor.Close()
	}
	return nil
}

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

	h.access.Lock()
	monitor := h.monitor
	current := make(map[string]*windowsExport, len(h.exports))
	for k, v := range h.exports {
		current[k] = v
	}
	h.access.Unlock()

	present := make(map[string]vboxusb.USBDeviceInfo, len(devices))
	keys := make([]DeviceKey, 0, len(devices))
	for _, d := range devices {
		present[d.BusID] = d
		key := DeviceKey{
			BusID:     d.BusID,
			VendorID:  d.VendorID,
			ProductID: d.ProductID,
		}
		if exp, ok := current[d.BusID]; ok && d.Captured && d.IdentityIsStub() {
			// The hub descriptor was unavailable and the registry reports
			// the VBox stub ID; evaluate the match against the identity
			// recorded when the export was created.
			key.VendorID = exp.info.VendorID
			key.ProductID = exp.info.ProductID
		}
		keys = append(keys, key)
	}
	desired := make(map[string]vboxusb.USBDeviceInfo)
	for _, idx := range SelectMatches(h.matches, keys) {
		info := present[keys[idx].BusID]
		if info.DeviceClass == 0x09 {
			h.logger.Warn("skip hub device ", info.BusID)
			continue
		}
		if info.Captured {
			if _, exported := current[info.BusID]; !exported {
				h.logger.Warn("device ", info.BusID, " is captured by VBoxUSB outside this service; replug it to export")
				continue
			}
		}
		desired[info.BusID] = info
	}

	now := time.Now()
	var released []string
	for busid, info := range desired {
		if exp, ok := current[busid]; ok {
			exp.markSeen(now, info.InstanceID, info.Captured)
			continue
		}
		exp := newWindowsExport(info, h.logger)
		exp.markSeen(now, info.InstanceID, info.Captured)
		h.captureDevice(monitor, busid, info)
		current[busid] = exp
		h.logger.Info("matched ", busid, " (vid=", fmt.Sprintf("0x%04x", info.VendorID), " pid=", fmt.Sprintf("0x%04x", info.ProductID), ") — capturing")
	}
	for busid, exp := range current {
		if _, ok := desired[busid]; ok {
			continue
		}
		if isReserved(busid) {
			continue
		}
		info, isPresent := present[busid]
		if !isPresent && now.Sub(exp.seenAt()) < windowsExportAbsenceGrace {
			// Likely mid-restart: the devnode is removed while PnP
			// re-enumerates it. Keep the export until the grace passes.
			continue
		}
		h.releaseDevice(monitor, busid, exp, info, isPresent)
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
	device := exp.takeDevice()
	if device != nil {
		_ = device.Close()
	}
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

// captureDevice installs a capture filter and forces the device through
// PnP re-enumeration. VBoxUSBMon only rewrites a device's IDs (handing
// it to VBoxUSB.sys) while the device enumerates, so without the
// restart an already-plugged device would keep its function driver
// until physically replugged.
func (h *windowsExportHost) captureDevice(monitor *vboxusb.Monitor, busid string, info vboxusb.USBDeviceInfo) {
	if monitor == nil {
		return
	}
	restart, err := vboxusb.BeginDeviceRestart(info.InstanceID, info.Address)
	if err != nil {
		h.logger.Warn("restart ", busid, " for capture: ", err)
	}
	vendor := info.VendorID
	product := info.ProductID
	filterID, err := monitor.AddFilter(vboxusb.Filter{
		VendorID:  &vendor,
		ProductID: &product,
	})
	if err != nil {
		h.logger.Warn("ADD_FILTER for ", busid, ": ", err)
	} else {
		h.access.Lock()
		h.filters[busid] = filterID
		h.access.Unlock()
	}
	if restart != nil {
		restart.Finish()
	}
}

// releaseDevice removes the capture filter and, if the device is still
// present and captured, restarts it so its original function driver
// re-binds. Without the restart the device stays dead to Windows until
// physically replugged.
func (h *windowsExportHost) releaseDevice(monitor *vboxusb.Monitor, busid string, exp *windowsExport, info vboxusb.USBDeviceInfo, isPresent bool) {
	device := exp.takeDevice()
	if device != nil {
		_ = device.Close()
	}
	var restart *vboxusb.DeviceRestart
	if isPresent && info.Captured {
		var err error
		restart, err = vboxusb.BeginDeviceRestart(info.InstanceID, info.Address)
		if err != nil {
			h.logger.Warn("restart ", busid, " for release: ", err)
		}
	}
	h.removeFilter(monitor, busid)
	if restart != nil {
		restart.Finish()
	}
}

func (h *windowsExportHost) removeFilter(monitor *vboxusb.Monitor, busid string) {
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

type windowsExport struct {
	info   vboxusb.USBDeviceInfo
	entry  DeviceEntry
	logger log.ContextLogger

	stateAccess       sync.Mutex
	device            *vboxusb.Device
	lastSeen          time.Time
	currentInstanceID string
	seenCaptured      bool
}

// setDevice records the claimed handle once NewServerDataSession opens it.
func (e *windowsExport) setDevice(device *vboxusb.Device) {
	e.stateAccess.Lock()
	e.device = device
	e.stateAccess.Unlock()
}

// takeDevice atomically hands the claimed handle to exactly one caller and
// clears the field, so FinishImport and Close racing on shutdown cannot both
// close the same handle.
func (e *windowsExport) takeDevice() *vboxusb.Device {
	e.stateAccess.Lock()
	device := e.device
	e.device = nil
	e.stateAccess.Unlock()
	return device
}

// markSeen records the device's enumeration state. The instance ID must be
// re-tracked every pass because capture rewrites it to the VBox stub ID.
func (e *windowsExport) markSeen(now time.Time, instanceID string, captured bool) {
	e.stateAccess.Lock()
	e.lastSeen = now
	e.currentInstanceID = instanceID
	e.seenCaptured = captured
	e.stateAccess.Unlock()
}

func (e *windowsExport) seenAt() time.Time {
	e.stateAccess.Lock()
	defer e.stateAccess.Unlock()
	return e.lastSeen
}

func (e *windowsExport) lastState() (string, bool) {
	e.stateAccess.Lock()
	defer e.stateAccess.Unlock()
	return e.currentInstanceID, e.seenCaptured
}

func newWindowsExport(info vboxusb.USBDeviceInfo, logger log.ContextLogger) *windowsExport {
	entry := DeviceEntry{
		Info: DeviceInfoTruncated{
			BusNum:             info.BusNumber,
			DevNum:             info.Address,
			Speed:              windowsSpeedToProtocol(info.Speed),
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

func windowsSpeedToProtocol(speed vboxusb.DeviceSpeed) uint32 {
	switch speed {
	case vboxusb.SpeedLow:
		return SpeedLow
	case vboxusb.SpeedFull:
		return SpeedFull
	case vboxusb.SpeedHigh:
		return SpeedHigh
	case vboxusb.SpeedSuper:
		return SpeedSuper
	case vboxusb.SpeedSuperPlus:
		return SpeedSuperPlus
	default:
		return SpeedUnknown
	}
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

func (e *windowsExport) NewServerDataSession(ctx context.Context, conn net.Conn) (DataSession, error) {
	path, err := vboxusb.WaitForCapturedDevice(e.info.BusNumber, e.info.Address, 10*time.Second)
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
	e.setDevice(device)
	return newUserspaceURBSession(ctx, e.logger, conn, newVBoxUSBEngine(device)), nil
}
