//go:build windows

package usbipvhci

import (
	"encoding/binary"
	"sync"
	"unsafe"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/windows"
)

// guidDevInterfaceUSBHostController is GUID_DEVINTERFACE_USB_HOST_CONTROLLER,
// registered by the usbip-win2 UDE driver for its emulated controller.
var guidDevInterfaceUSBHostController = windows.GUID{
	Data1: 0xB4030C06,
	Data2: 0xDC5F,
	Data3: 0x4FCC,
	Data4: [8]byte{0x87, 0xEB, 0xE5, 0x51, 0x5A, 0x09, 0x35, 0xC0},
}

// Controller is an open handle to the usbip-win2 VHCI device interface.
// One controller multiplexes every attached device. The handle is
// synchronous; access serializes through ioctlAccess so concurrent
// Plugin/Plugout calls do not interleave on the shared handle.
type Controller struct {
	handle      windows.Handle
	ioctlAccess sync.Mutex
	closing     sync.Once
	closeErr    error
}

// Open resolves and opens the VHCI device interface. It fails when the
// usbip-win2 driver is not installed (no interface present).
func Open() (*Controller, error) {
	path, err := interfacePath()
	if err != nil {
		return nil, err
	}
	pathW, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, E.Cause(err, "usbipvhci: utf16 device path")
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
		return nil, E.Cause(err, "usbipvhci: open ", path)
	}
	return &Controller{handle: handle}, nil
}

func (c *Controller) Close() error {
	c.closing.Do(func() {
		c.closeErr = windows.CloseHandle(c.handle)
		c.handle = windows.InvalidHandle
	})
	return c.closeErr
}

// Plugin attaches a remote device through the driver. host and service
// address the loopback relay sing-box runs; busid is the bus id the
// driver echoes back in its in-kernel OP_REQ_IMPORT, which sing-box must
// confirm in OP_REP_IMPORT. The call blocks until the driver has
// connected and finished the import handshake, then returns the assigned
// hub port (>= 1).
func (c *Controller) Plugin(host, service, busid string) (int, error) {
	var buf [pluginHardwareSize]byte
	binary.LittleEndian.PutUint32(buf[offsetPluginSize:], pluginHardwareSize)
	err := putCString(buf[offsetPluginBusID:offsetPluginBusID+BusIDSize], busid)
	if err != nil {
		return 0, E.Cause(err, "usbipvhci: busid")
	}
	err = putCString(buf[offsetPluginService:offsetPluginService+serviceSize], service)
	if err != nil {
		return 0, E.Cause(err, "usbipvhci: service")
	}
	err = putCString(buf[offsetPluginHost:offsetPluginHost+hostSize], host)
	if err != nil {
		return 0, E.Cause(err, "usbipvhci: host")
	}
	// The driver writes back size + port; read those 8 bytes into a
	// separate buffer (libusbip's outlen = offsetof(port) + sizeof(port)).
	var out [offsetPluginPort + 4]byte
	_, err = c.ioctl(ioctlPluginHardwareOnce, buf[:], out[:])
	if err != nil {
		return 0, E.Cause(err, "usbipvhci: PLUGIN_HARDWARE")
	}
	port := int(int32(binary.LittleEndian.Uint32(out[offsetPluginPort:])))
	if port <= 0 {
		return 0, E.New("usbipvhci: driver returned invalid port ", port)
	}
	return port, nil
}

// Plugout detaches the device on the given hub port. PortAll detaches
// every device. A port the driver already tore down (after its socket
// dropped) reports STATUS_DEVICE_NOT_CONNECTED, surfaced as an error.
func (c *Controller) Plugout(port int) error {
	var buf [plugoutHardwareSize]byte
	binary.LittleEndian.PutUint32(buf[0:], plugoutHardwareSize)
	binary.LittleEndian.PutUint32(buf[4:], uint32(int32(port)))
	_, err := c.ioctl(ioctlPlugoutHardware, buf[:], nil)
	if err != nil {
		return E.Cause(err, "usbipvhci: PLUGOUT_HARDWARE")
	}
	return nil
}

func (c *Controller) ioctl(code uint32, in, out []byte) (uint32, error) {
	var inPtr, outPtr *byte
	var inLen, outLen uint32
	if len(in) > 0 {
		inPtr = &in[0]
		inLen = uint32(len(in))
	}
	if len(out) > 0 {
		outPtr = &out[0]
		outLen = uint32(len(out))
	}
	c.ioctlAccess.Lock()
	defer c.ioctlAccess.Unlock()
	var returned uint32
	err := windows.DeviceIoControl(c.handle, code, inPtr, inLen, outPtr, outLen, &returned, nil)
	if err != nil {
		return 0, err
	}
	return returned, nil
}

func putCString(dst []byte, s string) error {
	if len(s) >= len(dst) {
		return E.New("value too long (", len(s), " >= ", len(dst), "): ", s)
	}
	copy(dst, s)
	dst[len(s)] = 0
	return nil
}

// interfacePath returns the device interface path for the VHCI
// controller. Passing a nil device id to CM_Get_Device_Interface_ListW
// enumerates every interface of the class, matching usbip-win2's
// userspace; x/sys/windows.CM_Get_Device_Interface_List cannot pass nil,
// so the two cfgmgr32 calls are issued directly.
func interfacePath() (string, error) {
	guid := guidDevInterfaceUSBHostController
	for {
		var size uint32
		ret, _, _ := procCMGetDeviceInterfaceListSizeW.Call(
			uintptr(unsafe.Pointer(&size)),
			uintptr(unsafe.Pointer(&guid)),
			0,
			uintptr(windows.CM_GET_DEVICE_INTERFACE_LIST_PRESENT),
		)
		if windows.CONFIGRET(ret) != windows.CR_SUCCESS {
			return "", E.New("usbipvhci: CM_Get_Device_Interface_List_Size CR=", ret)
		}
		if size <= 1 {
			return "", E.New("usbipvhci: VHCI device interface not found; is the usbip-win2 driver installed?")
		}
		buf := make([]uint16, size)
		ret, _, _ = procCMGetDeviceInterfaceListW.Call(
			uintptr(unsafe.Pointer(&guid)),
			0,
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(size),
			uintptr(windows.CM_GET_DEVICE_INTERFACE_LIST_PRESENT),
		)
		switch windows.CONFIGRET(ret) {
		case windows.CR_SUCCESS:
			path := windows.UTF16ToString(buf)
			if path == "" {
				return "", E.New("usbipvhci: VHCI device interface not found; is the usbip-win2 driver installed?")
			}
			return path, nil
		case windows.CR_BUFFER_SMALL:
			continue
		default:
			return "", E.New("usbipvhci: CM_Get_Device_Interface_List CR=", ret)
		}
	}
}

var (
	modCfgMgr32                       = windows.NewLazyDLL("cfgmgr32.dll")
	procCMGetDeviceInterfaceListSizeW = modCfgMgr32.NewProc("CM_Get_Device_Interface_List_SizeW")
	procCMGetDeviceInterfaceListW     = modCfgMgr32.NewProc("CM_Get_Device_Interface_ListW")
)
