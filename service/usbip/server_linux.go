//go:build linux

package usbip

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"

	"golang.org/x/sys/unix"
)

func newPlatformExportHost(logger log.ContextLogger, matches []option.USBIPDeviceMatch) (ExportHost, error) {
	return newLinuxExportHost(logger, matches, systemUSBIPOps), nil
}

func newPlatformImportHost(logger log.ContextLogger) (ImportHost, error) {
	return newLinuxImportHost(logger, systemUSBIPOps), nil
}

func linuxStableID(d sysfsDevice) string {
	if d.Serial != "" {
		return fmt.Sprintf("usb:%04x:%04x:%s", d.VendorID, d.ProductID, d.Serial)
	}
	return "linux-busid:" + d.BusID
}

func linuxUSBIPStatusState(status int) string {
	switch status {
	case usbipStatusAvailable:
		return deviceStateAvailable
	case usbipStatusUsed:
		return deviceStateBusy
	default:
		return deviceStateUnavailable
	}
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

func sysBusDevicePath(busid string) string {
	return sysBusUSBDevices + "/" + busid
}

func isVHCIImportedDevice(path string) bool {
	if strings.Contains(path, "vhci_hcd") {
		return true
	}
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	return strings.Contains(realPath, "vhci_hcd")
}

func isMissingUSBDeviceError(err error) bool {
	return errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ENODEV)
}

func driverOrNone(d string) string {
	if d == "" {
		return "(no driver)"
	}
	return d
}
