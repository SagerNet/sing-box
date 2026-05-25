//go:build windows

package vboxusb

import (
	"errors"
	"strconv"
	"strings"
	"time"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/windows"
)

var MonitorAccessGUID = windows.GUID{
	Data1: 0x00873fdf,
	Data2: 0xCAFE,
	Data3: 0x80EE,
	Data4: [8]byte{0xaa, 0x5e, 0x00, 0xc0, 0x4f, 0xb1, 0x72, 0x0b},
}

var USBDeviceInterfaceGUID = windows.GUID{
	Data1: 0xa5dcbf10,
	Data2: 0x6530,
	Data3: 0x11d2,
	Data4: [8]byte{0x90, 0x1f, 0x00, 0xc0, 0x4f, 0xb9, 0x51, 0xed},
}

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
	guid := USBDeviceInterfaceGUID
	devInfo, err := windows.SetupDiGetClassDevsEx(
		&guid,
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

func WaitForVBoxUSBInterface(instanceID string, timeout time.Duration) (string, error) {
	guid := MonitorAccessGUID
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
