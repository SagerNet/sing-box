// Package usbipvhci drives the usbip-win2 UDE (USB Device Emulation)
// virtual host controller on Windows. It lets sing-box's usbip-client
// service plug a remote device into the local USB stack: sing-box hands
// the driver a loopback endpoint and bridges it to the proxied server
// connection, so the in-kernel WSK socket only ever talks to 127.0.0.1
// while every byte still travels through sing-box's dialer.
//
// The driver is vadimgrn/usbip-win2 (BSD-2-Clause). sing-box ships the
// Microsoft-signed binaries unmodified.
//
// Upstream: https://github.com/vadimgrn/usbip-win2
// ABI mirrored from include/usbip/{vhci,consts}.h.
package usbipvhci

// Driver string-buffer sizes, from include/usbip/consts.h.
const (
	BusIDSize   = 32   // BUS_ID_SIZE
	serviceSize = 32   // NI_MAXSERV
	hostSize    = 1025 // NI_MAXHOST in ws2def.h
)

// IOCTL codes: CTL_CODE(FILE_DEVICE_UNKNOWN, function, METHOD_BUFFERED,
// FILE_READ_DATA|FILE_WRITE_DATA). Function values < 0x800 are reserved
// for Microsoft. PLUGIN_HARDWARE_ONCE only suppresses retries of the
// initial attach (vhci_ioctl.cpp checks one_attempt in the connect
// completion alone); when an established connection later drops,
// wsk_receive.cpp unconditionally schedules reattach attempts toward
// the by-then-dead one-shot loopback port, so every teardown must
// cancel them via STOP_ATTACH_ATTEMPTS.
const (
	ioctlPluginHardwareOnce uint32 = 0x0022_E018 // function 0x806
	ioctlPlugoutHardware    uint32 = 0x0022_E004 // function 0x801
	ioctlStopAttachAttempts uint32 = 0x0022_E014 // function 0x805
)

// Field offsets within usbip::vhci::ioctl::plugin_hardware, which is
// base{size} + imported_device_location{port,busid,service,host}. All
// members are int/char so the layout is dense.
const (
	offsetPluginSize    = 0
	offsetPluginPort    = 4
	offsetPluginBusID   = 8
	offsetPluginService = offsetPluginBusID + BusIDSize     // 40
	offsetPluginHost    = offsetPluginService + serviceSize // 72
)

// pluginHardwareSize is sizeof(plugin_hardware): 1097 raw bytes padded
// to the 4-byte struct alignment. The driver rejects any other size.
// The amd64 (0.9.7.7) and arm64 (0.9.7.5) drivers sing-box ships share
// this ABI, including the PLUGIN_HARDWARE_ONCE function.
const pluginHardwareSize = 1100

// plugoutHardwareSize is sizeof(plugout_hardware): ULONG size + int port.
const plugoutHardwareSize = 8

// stopAttachAttemptsSize is sizeof(ioctl::stop_attach_attempts):
// base + imported_device_location (1097) padded to 1100 for the
// trailing `int count` (OUT), total 1104. Present in both bundled
// driver versions; absent before 0.9.7.5.
const (
	stopAttachAttemptsSize        = 1104
	offsetStopAttachAttemptsCount = 1100
)

// PortAll detaches every imported device (PORT_ALL = -1).
const PortAll = -1

// udeHardwareID is the root-enumerated hardware id of the VHCI devnode,
// from usbip2_ude.inf.
const udeHardwareID = `ROOT\USBIP_WIN2\UDE`

// assetFile is one embedded driver-package file. assetFiles and
// assetVersion are defined per-architecture; assetVersion keys the
// on-disk extraction path so a driver bump re-extracts.
type assetFile struct {
	name string
	data []byte
}
