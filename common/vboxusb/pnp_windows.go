//go:build windows

package vboxusb

import (
	"encoding/binary"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/windows"
)

// USBDeviceInfo describes one USB device enumerated by Windows
// regardless of which function driver currently owns it. Bus/Address
// values are normalized into a stable bus-id string ("<bus>-<address>")
// matching Linux usbip conventions.
type USBDeviceInfo struct {
	InstanceID  string
	HardwareID  string
	VendorID    uint16
	ProductID   uint16
	Revision    uint16
	BusNumber   uint32
	Address     uint32 // device address on the bus (port path leaf)
	BusID       string // "<bus>-<address>"
	DeviceClass uint8
}

// EnumerateUSBDevices walks GUID_DEVINTERFACE_USB_DEVICE and returns
// one record per attached USB device. Hubs are not filtered out here;
// the caller (export host) skips DeviceClass == 0x09.
func EnumerateUSBDevices() ([]USBDeviceInfo, error) {
	devInfo, err := windows.SetupDiGetClassDevsEx(
		&windows.GUID{
			Data1: USBDeviceInterfaceGUID.Data1,
			Data2: USBDeviceInterfaceGUID.Data2,
			Data3: USBDeviceInterfaceGUID.Data3,
			Data4: USBDeviceInterfaceGUID.Data4,
		},
		"",
		0,
		windows.DIGCF_PRESENT|windows.DIGCF_DEVICEINTERFACE,
		0,
		"",
	)
	if err != nil {
		return nil, E.Cause(err, "vboxusb: SetupDiGetClassDevsEx")
	}
	defer devInfo.Close()

	var out []USBDeviceInfo
	for i := 0; ; i++ {
		data, err := windows.SetupDiEnumDeviceInfo(devInfo, i)
		if err != nil {
			if errors.Is(err, windows.ERROR_NO_MORE_ITEMS) {
				break
			}
			return nil, E.Cause(err, "vboxusb: SetupDiEnumDeviceInfo[", i, "]")
		}
		info := USBDeviceInfo{}
		info.InstanceID, err = windows.SetupDiGetDeviceInstanceId(devInfo, data)
		if err != nil {
			continue
		}
		hardwareIDValue, err := windows.SetupDiGetDeviceRegistryProperty(devInfo, data, windows.SPDRP_HARDWAREID)
		if err == nil {
			info.HardwareID = firstString(hardwareIDValue)
			info.VendorID, info.ProductID, info.Revision = parseHardwareID(info.HardwareID)
		}
		busNumberValue, err := windows.SetupDiGetDeviceRegistryProperty(devInfo, data, windows.SPDRP_BUSNUMBER)
		if err == nil {
			info.BusNumber = toUint32(busNumberValue)
		}
		addressValue, err := windows.SetupDiGetDeviceRegistryProperty(devInfo, data, windows.SPDRP_ADDRESS)
		if err == nil {
			info.Address = toUint32(addressValue)
		}
		info.BusID = strconv.FormatUint(uint64(info.BusNumber), 10) + "-" + strconv.FormatUint(uint64(info.Address), 10)
		out = append(out, info)
	}
	return out, nil
}

// CycleHubPort issues IOCTL_USB_HUB_CYCLE_PORT on the parent hub for
// the given 1-based port number. This is the supported equivalent of
// physically unplugging and replugging the device; combined with a
// VBoxUSBMon filter it triggers PnP to bind VBoxUSB to the device.
//
// hubInterfacePath is the setupapi-resolved interface path of the
// parent hub (obtained via CM_Get_Device_Interface_List with
// USBHubInterfaceGUID).
func CycleHubPort(hubInterfacePath string, port uint32) error {
	pathW, err := windows.UTF16PtrFromString(hubInterfacePath)
	if err != nil {
		return E.Cause(err, "vboxusb: utf16 hub path")
	}
	handle, err := windows.CreateFile(
		pathW,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return E.Cause(err, "vboxusb: open hub ", hubInterfacePath)
	}
	defer windows.CloseHandle(handle)

	// USB_CYCLE_PORT_PARAMS: ULONG ConnectionIndex; ULONG StatusReturned;
	var params [8]byte
	binary.LittleEndian.PutUint32(params[0:4], port)
	var returned uint32
	err = windows.DeviceIoControl(
		handle,
		IOCTLHubCyclePort,
		&params[0], uint32(len(params)),
		&params[0], uint32(len(params)),
		&returned, nil,
	)
	if err != nil {
		return E.Cause(err, "vboxusb: IOCTL_USB_HUB_CYCLE_PORT")
	}
	return nil
}

// WaitForVBoxUSBInterface polls CM_Get_Device_Interface_List for the
// VBoxUSB class GUID until an interface path matching instanceID
// appears, or timeout elapses. After RestartDevice + filter trigger,
// PnP needs a moment to load VBoxUSB.sys on the device; matching
// usbipd-win's 10-second window.
func WaitForVBoxUSBInterface(instanceID string, timeout time.Duration) (string, error) {
	guid := windows.GUID{
		Data1: MonitorAccessGUID.Data1,
		Data2: MonitorAccessGUID.Data2,
		Data3: MonitorAccessGUID.Data3,
		Data4: MonitorAccessGUID.Data4,
	}
	deadline := time.Now().Add(timeout)
	for {
		paths, err := windows.CM_Get_Device_Interface_List(instanceID, &guid, windows.CM_GET_DEVICE_INTERFACE_LIST_PRESENT)
		if err == nil {
			for _, p := range paths {
				if p != "" {
					return p, nil
				}
			}
		}
		if time.Now().After(deadline) {
			return "", E.New("vboxusb: VBoxUSB interface for ", instanceID, " did not appear within ", timeout)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// RestartDevice triggers the PnP unplug/replug cycle that lets a
// pending VBoxUSBMon CAPTURE filter activate. usbipd-win uses
// CM_Query_And_Remove_SubTree -> IOCTL_USB_HUB_CYCLE_PORT ->
// CM_Setup_DevNode.
//
// TODO(phase B follow-up): CM_Query_And_Remove_SubTree and
// CM_Setup_DevNode are not exposed by golang.org/x/sys/windows and
// need direct LazyDLL wrappers. Until that is in place, callers
// should arrange for the device to be physically replugged.
func RestartDevice(_ string) error {
	return E.New("vboxusb: RestartDevice not yet implemented (use CycleHubPort for now)")
}

// WatchDeviceArrival registers a callback for device arrival/removal
// on GUID_DEVINTERFACE_USB_DEVICE. Returns a handle that must be
// closed when the watcher is no longer needed.
//
// TODO(phase B follow-up): CM_Register_Notification is not exposed by
// golang.org/x/sys/windows. Until wired up, the export host will need
// to poll via EnumerateUSBDevices on a reconcile timer.
func WatchDeviceArrival(_ func()) (DeviceWatcher, error) {
	return nil, E.New("vboxusb: WatchDeviceArrival not yet implemented (poll EnumerateUSBDevices instead)")
}

// DeviceWatcher is the handle returned by WatchDeviceArrival. Close
// stops delivery and releases the underlying CM_Notify_HNOTIFICATION.
type DeviceWatcher interface {
	Close() error
}

// firstString returns the first NUL-separated string in a REG_MULTI_SZ
// value (returned by SetupDiGetDeviceRegistryProperty for HardwareID).
// Returns the value unchanged for REG_SZ.
func firstString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []string:
		if len(v) > 0 {
			return v[0]
		}
	}
	return ""
}

func toUint32(value any) uint32 {
	switch v := value.(type) {
	case uint32:
		return v
	case uint64:
		return uint32(v)
	}
	return 0
}

// parseHardwareID extracts VID/PID/REV from a USB hardware ID such as
// "USB\VID_046D&PID_C31C&REV_6400". Returns zeros on parse failure;
// the caller decides whether that disqualifies the device.
func parseHardwareID(hwid string) (vid, pid, rev uint16) {
	upper := strings.ToUpper(hwid)
	vid = extractHex16(upper, "VID_")
	pid = extractHex16(upper, "PID_")
	rev = extractHex16(upper, "REV_")
	return
}

func extractHex16(s, prefix string) uint16 {
	idx := strings.Index(s, prefix)
	if idx < 0 {
		return 0
	}
	tail := s[idx+len(prefix):]
	end := len(tail)
	for i, r := range tail {
		if !isHex(r) {
			end = i
			break
		}
	}
	if end == 0 {
		return 0
	}
	v, err := strconv.ParseUint(tail[:end], 16, 16)
	if err != nil {
		return 0
	}
	return uint16(v)
}

func isHex(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'A' && r <= 'F') || (r >= 'a' && r <= 'f')
}

// pnpProcOnce + LazyProc handles for direct syscall to functions that
// golang.org/x/sys/windows does not yet wrap. Kept in one place so the
// RestartDevice / WatchDeviceArrival follow-up has the resolver
// boilerplate already laid out.
var (
	pnpProcOnce sync.Once
	modCfgMgr32 = windows.NewLazyDLL("cfgmgr32.dll")
	_           = modCfgMgr32 // referenced by upcoming RestartDevice impl
	_           unsafe.Pointer
)
