//go:build windows

package vboxusb

import (
	"encoding/binary"

	"golang.org/x/sys/windows"
)

// DeviceSpeed is the negotiated USB link speed of a device, independent of the
// USB/IP wire encoding. The export host maps it to the protocol speed code.
type DeviceSpeed uint8

const (
	SpeedUnknown DeviceSpeed = iota
	SpeedLow
	SpeedFull
	SpeedHigh
	SpeedSuper
	SpeedSuperPlus
)

// GUID_DEVINTERFACE_USB_HUB.
var usbHubInterfaceGUID = windows.GUID{
	Data1: 0xf18a0e88,
	Data2: 0xc30c,
	Data3: 0x11d0,
	Data4: [8]byte{0x88, 0x15, 0x00, 0xa0, 0xc9, 0x06, 0xbe, 0xd8},
}

// DEVPKEY_Device_Parent: the parent device instance id (the hub a device
// hangs off). Reported as a DEVPROP_TYPE_STRING.
var devpkeyDeviceParent = windows.DEVPROPKEY{
	FmtID: windows.DEVPROPGUID{
		Data1: 0x4340a6c5,
		Data2: 0x93fa,
		Data3: 0x4706,
		Data4: [8]byte{0x97, 0x2c, 0x7b, 0x64, 0x80, 0x08, 0xa5, 0xa7},
	},
	PID: 8,
}

const (
	ioctlUSBGetNodeConnectionInformationEx   uint32 = 0x0022_0448
	ioctlUSBGetNodeConnectionInformationExV2 uint32 = 0x0022_048C

	// Offset of the Speed byte inside USB_NODE_CONNECTION_INFORMATION_EX:
	// ConnectionIndex(4) + USB_DEVICE_DESCRIPTOR(18) + CurrentConfigurationValue(1).
	nodeConnInfoExSpeedOffset = 23
	// Fixed part is 36 bytes; the IOCTL also appends one USB_PIPE_INFO per open
	// pipe and fails if the buffer is too small, so allow for a fully configured
	// device's pipe list.
	nodeConnInfoExBufferSize = 2048

	// USB_DEVICE_SPEED values reported by the EX IOCTL.
	usbDeviceSpeedLow   = 0
	usbDeviceSpeedFull  = 1
	usbDeviceSpeedHigh  = 2
	usbDeviceSpeedSuper = 3

	// USB_NODE_CONNECTION_INFORMATION_EX_V2: ConnectionIndex(4) + Length(4) +
	// SupportedUsbProtocols(4) + Flags(4).
	nodeConnInfoExV2Size      = 16
	nodeConnInfoExV2FlagsOff  = 12
	nodeConnInfoExV2LengthOff = 4
	// USB_NODE_CONNECTION_INFORMATION_EX_V2_FLAGS bit
	// DeviceIsOperatingAtSuperSpeedPlusOrHigher.
	nodeConnInfoExV2FlagSuperSpeedPlus = 0x4
)

// hubSpeedProbe resolves the negotiated link speed of devices by querying their
// parent hub. Open hub handles are cached for the lifetime of one enumeration;
// a nil/InvalidHandle entry caches a failure so it is not retried per device.
type hubSpeedProbe struct {
	hubs map[string]windows.Handle
}

func newHubSpeedProbe() *hubSpeedProbe {
	return &hubSpeedProbe{hubs: make(map[string]windows.Handle)}
}

func (p *hubSpeedProbe) close() {
	for _, handle := range p.hubs {
		if handle != windows.InvalidHandle {
			_ = windows.CloseHandle(handle)
		}
	}
	p.hubs = nil
}

// speedOf returns the device's link speed, or SpeedUnknown if the parent hub
// could not be opened or did not answer. port is the hub port index
// (SPDRP_ADDRESS) the device is attached to.
func (p *hubSpeedProbe) speedOf(devInfo windows.DevInfo, data *windows.DevInfoData, port uint32) DeviceSpeed {
	hub := p.parentHub(devInfo, data)
	if hub == windows.InvalidHandle {
		return SpeedUnknown
	}
	return querySpeed(hub, port)
}

func (p *hubSpeedProbe) parentHub(devInfo windows.DevInfo, data *windows.DevInfoData) windows.Handle {
	parentValue, err := windows.SetupDiGetDeviceProperty(devInfo, data, &devpkeyDeviceParent)
	if err != nil {
		return windows.InvalidHandle
	}
	parentID, isString := parentValue.(string)
	if !isString || parentID == "" {
		return windows.InvalidHandle
	}
	paths, err := windows.CM_Get_Device_Interface_List(parentID, &usbHubInterfaceGUID, windows.CM_GET_DEVICE_INTERFACE_LIST_PRESENT)
	if err != nil {
		return windows.InvalidHandle
	}
	var hubPath string
	for _, candidate := range paths {
		if candidate != "" {
			hubPath = candidate
			break
		}
	}
	if hubPath == "" {
		return windows.InvalidHandle
	}
	cached, found := p.hubs[hubPath]
	if found {
		return cached
	}
	handle := openHub(hubPath)
	p.hubs[hubPath] = handle
	return handle
}

func openHub(hubPath string) windows.Handle {
	pathUTF16, err := windows.UTF16PtrFromString(hubPath)
	if err != nil {
		return windows.InvalidHandle
	}
	handle, err := windows.CreateFile(pathUTF16, windows.GENERIC_WRITE, windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return windows.InvalidHandle
	}
	return handle
}

func querySpeed(hub windows.Handle, port uint32) DeviceSpeed {
	buffer := make([]byte, nodeConnInfoExBufferSize)
	binary.LittleEndian.PutUint32(buffer[0:4], port)
	var returned uint32
	err := windows.DeviceIoControl(
		hub,
		ioctlUSBGetNodeConnectionInformationEx,
		&buffer[0], uint32(len(buffer)),
		&buffer[0], uint32(len(buffer)),
		&returned, nil,
	)
	if err != nil || returned <= nodeConnInfoExSpeedOffset {
		return SpeedUnknown
	}
	switch buffer[nodeConnInfoExSpeedOffset] {
	case usbDeviceSpeedLow:
		return SpeedLow
	case usbDeviceSpeedFull:
		return SpeedFull
	case usbDeviceSpeedHigh:
		return SpeedHigh
	case usbDeviceSpeedSuper:
		if superSpeedPlus(hub, port) {
			return SpeedSuperPlus
		}
		return SpeedSuper
	default:
		return SpeedUnknown
	}
}

func superSpeedPlus(hub windows.Handle, port uint32) bool {
	var buffer [nodeConnInfoExV2Size]byte
	binary.LittleEndian.PutUint32(buffer[0:4], port)
	binary.LittleEndian.PutUint32(buffer[nodeConnInfoExV2LengthOff:], nodeConnInfoExV2Size)
	var returned uint32
	err := windows.DeviceIoControl(
		hub,
		ioctlUSBGetNodeConnectionInformationExV2,
		&buffer[0], uint32(len(buffer)),
		&buffer[0], uint32(len(buffer)),
		&returned, nil,
	)
	if err != nil || returned < nodeConnInfoExV2Size {
		return false
	}
	flags := binary.LittleEndian.Uint32(buffer[nodeConnInfoExV2FlagsOff:])
	return flags&nodeConnInfoExV2FlagSuperSpeedPlus != 0
}
