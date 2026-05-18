//go:build windows

package vboxusb

import (
	"encoding/binary"
	"errors"
	"sync"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/windows"
)

// Monitor is a handle to \\.\VBoxUSBMon. It is shared across the
// process (one global monitor handle per sing-box server); per-device
// filters are added/removed against it. Concurrent ADD_FILTER /
// REMOVE_FILTER calls are serialized inside the driver but our handle
// reuses a single overlapped event — so Monitor methods are not safe
// for concurrent use across goroutines. The caller (the export host)
// is expected to serialize.
type Monitor struct {
	handle   windows.Handle
	event    windows.Handle
	closing  sync.Once
	closeErr error
}

// OpenMonitor opens \\.\VBoxUSBMon. EnsureDrivers must have been
// called first (and must have succeeded) so the VBoxUSBMon kernel
// service is loaded; otherwise CreateFile returns FILE_NOT_FOUND.
func OpenMonitor() (*Monitor, error) {
	pathW, err := windows.UTF16PtrFromString(MonitorDevicePath)
	if err != nil {
		return nil, E.Cause(err, "vboxusb: utf16 monitor path")
	}
	handle, err := windows.CreateFile(
		pathW,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OVERLAPPED,
		0,
	)
	if err != nil {
		if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
			return nil, E.Cause(err, "vboxusb: open monitor (driver not loaded?)")
		}
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			return nil, E.Cause(err, "vboxusb: open monitor (administrator required)")
		}
		return nil, E.Cause(err, "vboxusb: open monitor")
	}
	event, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		windows.CloseHandle(handle)
		return nil, E.Cause(err, "vboxusb: create monitor event")
	}
	return &Monitor{handle: handle, event: event}, nil
}

func (m *Monitor) Close() error {
	m.closing.Do(func() {
		var errs []error
		if m.handle != 0 {
			err := windows.CloseHandle(m.handle)
			if err != nil {
				errs = append(errs, err)
			}
			m.handle = 0
		}
		if m.event != 0 {
			err := windows.CloseHandle(m.event)
			if err != nil {
				errs = append(errs, err)
			}
			m.event = 0
		}
		m.closeErr = E.Errors(errs...)
	})
	return m.closeErr
}

// GetVersion returns the monitor driver version (8 bytes packed:
// major + minor). Reject if major mismatches DriverMajorVersion.
func (m *Monitor) GetVersion() (uint32, uint32, error) {
	var buf [8]byte
	_, err := m.ioctl(IOCTLMonitorGetVersion, nil, buf[:])
	if err != nil {
		return 0, 0, E.Cause(err, "vboxusb: monitor GET_VERSION")
	}
	return binary.LittleEndian.Uint32(buf[0:4]), binary.LittleEndian.Uint32(buf[4:8]), nil
}

// AddFilter installs a one-shot filter that captures the next PnP
// arrival matching the filter. Returns a filter id that must be passed
// to RemoveFilter on detach. usbipd-win uses a permanent CAPTURE
// filter; we follow the same pattern so the filter survives transient
// PnP retries during RestartingDevice.
func (m *Monitor) AddFilter(filter Filter) (uint64, error) {
	in := encodeFilter(filter)
	var out [12]byte // UsbSupFltAddOut: uint64 uId + int32 rc
	_, err := m.ioctl(IOCTLMonitorAddFilter, in[:], out[:])
	if err != nil {
		return 0, E.Cause(err, "vboxusb: monitor ADD_FILTER")
	}
	id := binary.LittleEndian.Uint64(out[0:8])
	rc := int32(binary.LittleEndian.Uint32(out[8:12]))
	if rc < 0 {
		return 0, E.New("vboxusb: monitor ADD_FILTER returned rc=", rc)
	}
	return id, nil
}

// RemoveFilter releases a filter previously returned by AddFilter.
// Safe to call after the device has already left.
func (m *Monitor) RemoveFilter(id uint64) error {
	var in [8]byte
	binary.LittleEndian.PutUint64(in[:], id)
	_, err := m.ioctl(IOCTLMonitorRemoveFilter, in[:], nil)
	if err != nil {
		return E.Cause(err, "vboxusb: monitor REMOVE_FILTER")
	}
	return nil
}

func (m *Monitor) ioctl(code uint32, in []byte, out []byte) (uint32, error) {
	var overlapped windows.Overlapped
	overlapped.HEvent = m.event
	_ = windows.ResetEvent(m.event)
	var inPtr *byte
	var inLen uint32
	if len(in) > 0 {
		inPtr = &in[0]
		inLen = uint32(len(in))
	}
	var outPtr *byte
	var outLen uint32
	if len(out) > 0 {
		outPtr = &out[0]
		outLen = uint32(len(out))
	}
	var returned uint32
	err := windows.DeviceIoControl(m.handle, code, inPtr, inLen, outPtr, outLen, &returned, &overlapped)
	if err != nil && !errors.Is(err, windows.ERROR_IO_PENDING) {
		return 0, err
	}
	err = windows.GetOverlappedResult(m.handle, &overlapped, &returned, true)
	if err != nil {
		return 0, err
	}
	return returned, nil
}

// encodeFilter builds a 312-byte USBFILTER packed struct matching the
// layout in VirtualBox usbfilter.h (also documented in
// /tmp/usbipd-win/Usbipd/Interop/VBoxUsbMon.cs:77-105).
//
// Layout (offsets):
//
//	 0 u32Magic    uint32  (0x19670408)
//	 4 enmType     uint32  (5 = CAPTURE)
//	 8 aFields     [11]{enmMatch uint16, u16Value uint16}  (44 bytes)
//	52 offCurEnd   uint32  (0)
//	56 achStrTab   [256]byte  (0)
//
// All entries default to IGNORE; caller-specified fields are upgraded
// to NUM_EXACT with the supplied value. String matches and offCurEnd
// stay zero (we never use string filters; the driver rejects nonzero
// offCurEnd with strange offsets).
func encodeFilter(f Filter) [312]byte {
	const (
		filterMagic   uint32 = 0x19670408
		filterCapture uint32 = 5 // UsbFilterType.CAPTURE
		matchIgnore   uint16 = 1 // UsbFilterMatch.IGNORE
		matchNumExact uint16 = 3 // UsbFilterMatch.NUM_EXACT
	)
	const (
		idxVendorID    = 0
		idxProductID   = 1
		idxDeviceRev   = 2
		idxDeviceClass = 3
		idxBus         = 6
		idxPort        = 7
	)
	var raw [312]byte
	binary.LittleEndian.PutUint32(raw[0:4], filterMagic)
	binary.LittleEndian.PutUint32(raw[4:8], filterCapture)
	for i := 0; i < 11; i++ {
		binary.LittleEndian.PutUint16(raw[8+i*4:8+i*4+2], matchIgnore)
	}
	setField := func(idx int, value uint16) {
		binary.LittleEndian.PutUint16(raw[8+idx*4:8+idx*4+2], matchNumExact)
		binary.LittleEndian.PutUint16(raw[8+idx*4+2:8+idx*4+4], value)
	}
	if f.VendorID != nil {
		setField(idxVendorID, *f.VendorID)
	}
	if f.ProductID != nil {
		setField(idxProductID, *f.ProductID)
	}
	if f.DeviceRev != nil {
		setField(idxDeviceRev, *f.DeviceRev)
	}
	if f.DeviceClass != nil {
		setField(idxDeviceClass, *f.DeviceClass)
	}
	if f.Bus != nil {
		setField(idxBus, *f.Bus)
	}
	if f.Port != nil {
		setField(idxPort, *f.Port)
	}
	return raw
}
