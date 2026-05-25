// Package vboxusb provides a Go binding for Oracle's VBoxUSB +
// VBoxUSBMon kernel drivers on Windows (amd64 and arm64), packaged
// for embedding in sing-box.
//
// The driver pair is shipped verbatim from dorssel/usbipd-win, which
// in turn carries the binaries unchanged from upstream VirtualBox.
// VBoxUSB.sys provides per-device user-space URB submission via
// SUPUSB_IOCTL_* IOCTLs; VBoxUSBMon.sys is the system-wide monitor
// that arranges for VBoxUSB to bind matching devices on PnP arrival.
//
// Administrator is required for the first Open in a process so SCM
// can load VBoxUSBMon. The drivers are Microsoft-signed via Oracle's
// VirtualBox publishing chain; we never re-sign or modify them.
//
// Upstream:
//   - https://github.com/dorssel/usbipd-win (driver packaging)
//   - https://github.com/VirtualBox/VirtualBox (driver source)
//
// License: GPLv2-or-later (drivers) and GPLv3 (this binding); see
// assets/*.license files for the per-asset SPDX records.
package vboxusb

// AssetVersion identifies the bundled VBoxUSB driver release. Bumped
// in lock-step with the .sys files copied from
// /tmp/usbipd-win/Drivers/{x64,arm64}/. The on-disk extraction path
// (and SCM service version reuse) is keyed on this string, so a
// version bump triggers re-extraction.
const AssetVersion = "7.2.8.23730"

// DriverVersion enforces the minimum acceptable VBoxUSB/VBoxUSBMon
// driver major version reported via GET_VERSION. Mirrors usbipd-win's
// USBDRV_MAJOR_VERSION / USBMON_MAJOR_VERSION (both 5).
const (
	DriverMajorVersion = 5
	DriverMinorVersion = 0
)

// Driver and device names. The Windows side opens the monitor via
// CreateFile(MonitorDevicePath); per-device VBoxUSB handles are opened
// via SetupDi-resolved interface paths under the GUID below.
const (
	MonitorServiceName = "VBoxUSBMon"
	MonitorDevicePath  = `\\.\VBoxUSBMon`
)

// IOCTL codes from VirtualBox usblib-win.h, identical to those used by
// usbipd-win (Usbipd/Interop/VBoxUsb.cs:26-39 and VBoxUsbMon.cs:122-129).
// Encoding is the standard CTL_CODE shape:
//
//	(DeviceType << 16) | (Access << 14) | (Function << 2) | Method
//
// DeviceType = FILE_DEVICE_UNKNOWN (0x22), Access = FILE_WRITE_ACCESS (2),
// Method = METHOD_BUFFERED (0).
const (
	// Per-device VBoxUSB.sys (\\?\<setupapi-resolved path>).
	IOCTLSendURB            uint32 = 0x0022_181C // function 0x607
	IOCTLUSBSelectInterface uint32 = 0x0022_1824 // function 0x609
	IOCTLUSBSetConfig       uint32 = 0x0022_1828 // function 0x60a
	IOCTLUSBClaimDevice     uint32 = 0x0022_182C // function 0x60b
	IOCTLUSBClearEndpoint   uint32 = 0x0022_1838 // function 0x60e
	IOCTLGetVersion         uint32 = 0x0022_183C // function 0x60f
	IOCTLUSBAbortEndpoint   uint32 = 0x0022_1840 // function 0x610

	// VBoxUSBMon (\\.\VBoxUSBMon). Note GET_VERSION shares the numeric
	// code with VBoxUSB's USB_ABORT_ENDPOINT — different handles.
	IOCTLMonitorGetVersion   uint32 = 0x0022_1840
	IOCTLMonitorAddFilter    uint32 = 0x0022_1844
	IOCTLMonitorRemoveFilter uint32 = 0x0022_1848
)

// USB/IP-style transfer type enum, matching VirtualBox USBSUP_TRANSFER_TYPE.
type TransferType uint32

const (
	TransferTypeControl TransferType = iota
	TransferTypeIso
	TransferTypeBulk
	TransferTypeInterrupt
	TransferTypeMessage // control with setup packet inline
)

// Direction matches USBSUP_DIRECTION.
type Direction uint32

const (
	DirectionSetup Direction = iota
	DirectionIn
	DirectionOut
)

// TransferFlags matches USBSUP_XFER_FLAG. ShortOK is required for IN
// transfers unless the USB/IP request flags include URB_SHORT_NOT_OK.
type TransferFlags uint32

const (
	TransferFlagNone    TransferFlags = 0
	TransferFlagShortOK TransferFlags = 1 << 0
)

// URBError mirrors USBSUP_ERROR. The session layer maps these into
// USBIP-wire-format status (negated Linux errno).
type URBError uint32

const (
	URBOK URBError = iota
	URBStall
	URBDeviceNotResponding
	URBCRCError
	URBNACError
	URBUnderrun
	URBOverrun
)

// MaxIsoPacketsPerURB is the hard VBoxUSB limit (USBSUP_URB.aIsoPkts
// is sized for 8 entries). Callers with more iso packets must split
// into multiple URBs sharing one pinned buffer; offsets must stay
// within ushort range.
const MaxIsoPacketsPerURB = 8

// Filter is a logical builder for VBoxUSBMon ADD_FILTER. The Go side
// owns the byte layout (in monitor_windows.go) so callers see a clean
// API even though the on-the-wire struct is fixed-size packed.
type Filter struct {
	VendorID    *uint16
	ProductID   *uint16
	DeviceRev   *uint16
	Bus         *uint16
	Port        *uint16
	DeviceClass *uint16
}
