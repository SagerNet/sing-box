//go:build linux

package usbip

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/shell"
)

const (
	sysBusUSBDevices    = "/sys/bus/usb/devices"
	sysUsbipHostDriver  = "/sys/bus/usb/drivers/usbip-host"
	sysVHCIControllerV0 = "/sys/devices/platform/vhci_hcd.0"

	usbipStatusAvailable = 1
	usbipStatusUsed      = 2
	usbipStatusError     = 3
)

type sysfsDevice struct {
	BusID          string
	Path           string
	BusNum         uint32
	DevNum         uint32
	Speed          uint32
	VendorID       uint16
	ProductID      uint16
	BCDDevice      uint16
	DeviceClass    uint8
	DeviceSubClass uint8
	DeviceProtocol uint8
	ConfigValue    uint8
	NumConfigs     uint8
	NumInterfaces  uint8
	Serial         string
	Interfaces     []DeviceInterface
}

func (d *sysfsDevice) toProtocol() DeviceInfoTruncated {
	var info DeviceInfoTruncated
	encodePathField(&info.Path, d.Path, d.Serial)
	copy(info.BusID[:], d.BusID)
	info.BusNum = d.BusNum
	info.DevNum = d.DevNum
	info.Speed = d.Speed
	info.IDVendor = d.VendorID
	info.IDProduct = d.ProductID
	info.BCDDevice = d.BCDDevice
	info.BDeviceClass = d.DeviceClass
	info.BDeviceSubClass = d.DeviceSubClass
	info.BDeviceProtocol = d.DeviceProtocol
	info.BConfigurationValue = d.ConfigValue
	info.BNumConfigurations = d.NumConfigs
	info.BNumInterfaces = d.NumInterfaces
	return info
}

// vhciStatusRecord is one row of /sys/devices/platform/vhci_hcd.0/status
// or status.N. The kernel emits globally unique port numbers across every
// status* file; secondary identifies which file the row came from
// (0 for status, N for status.N) and exists only for diagnostic logging.
type vhciStatusRecord struct {
	secondary int
	hub       string
	port      int
	state     int
}

func listUSBDevices() ([]sysfsDevice, error) {
	entries, err := os.ReadDir(sysBusUSBDevices)
	if err != nil {
		return nil, err
	}
	var devices []sysfsDevice
	for _, entry := range entries {
		name := entry.Name()
		if strings.Contains(name, ":") {
			continue
		}
		path := filepath.Join(sysBusUSBDevices, name)
		device, err := readSysfsDevice(name, path)
		if err != nil {
			continue
		}
		devices = append(devices, device)
	}
	return devices, nil
}

func readSysfsDevice(busid, path string) (sysfsDevice, error) {
	d := sysfsDevice{BusID: busid, Path: path}
	vendor, err := readHexU16(path, "idVendor")
	if err != nil {
		return d, err
	}
	d.VendorID = vendor
	d.ProductID, _ = readHexU16(path, "idProduct")
	d.BCDDevice, _ = readHexU16(path, "bcdDevice")
	if v, err := readDecU32(path, "busnum"); err == nil {
		d.BusNum = v
	}
	if v, err := readDecU32(path, "devnum"); err == nil {
		d.DevNum = v
	}
	d.Speed = speedCodeFromString(readString(path, "speed"))
	d.DeviceClass, _ = readHexU8(path, "bDeviceClass")
	d.DeviceSubClass, _ = readHexU8(path, "bDeviceSubClass")
	d.DeviceProtocol, _ = readHexU8(path, "bDeviceProtocol")
	d.ConfigValue, _ = readDecU8(path, "bConfigurationValue")
	d.NumConfigs, _ = readDecU8(path, "bNumConfigurations")
	d.NumInterfaces, _ = readDecU8(path, "bNumInterfaces")
	d.Serial = readString(path, "serial")
	d.Interfaces = readInterfaces(path, busid, d.ConfigValue, int(d.NumInterfaces))
	return d, nil
}

func readInterfaces(devicePath, busid string, configValue uint8, count int) []DeviceInterface {
	if count == 0 {
		return nil
	}
	interfaces := make([]DeviceInterface, count)
	for i := range count {
		name := fmt.Sprintf("%s:%d.%d", busid, configValue, i)
		ipath := filepath.Join(filepath.Dir(devicePath), name)
		class, _ := readHexU8(ipath, "bInterfaceClass")
		subClass, _ := readHexU8(ipath, "bInterfaceSubClass")
		protocol, _ := readHexU8(ipath, "bInterfaceProtocol")
		interfaces[i] = DeviceInterface{
			BInterfaceClass:    class,
			BInterfaceSubClass: subClass,
			BInterfaceProtocol: protocol,
		}
	}
	return interfaces
}

func currentDriver(busid string) (string, error) {
	link, err := os.Readlink(filepath.Join(sysBusUSBDevices, busid, "driver"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return filepath.Base(link), nil
}

func reloadHostDriver() error {
	modprobePath, err := findModprobePath()
	if err != nil {
		return err
	}
	output, err := shell.Exec(modprobePath, "-r", "usbip-host").Read()
	if err != nil {
		return E.Extend(E.Cause(err, "unload kernel module usbip-host"), strings.TrimSpace(output))
	}
	return ensureKernelPath(sysUsbipHostDriver, "usbip-host", "usbip-host driver")
}

func readUsbipStatus(busid string) (int, error) {
	raw, err := os.ReadFile(filepath.Join(sysBusUSBDevices, busid, "usbip_status"))
	if err != nil {
		return 0, err
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return 0, err
	}
	return v, nil
}

// finishImportStatusTimeout is the upper bound for waitForUsbipStatusCleared.
// It is a var (not const) so interop tests can shrink it without changing the
// polling cadence.
var finishImportStatusTimeout = 2 * time.Second

const finishImportStatusPollInterval = 25 * time.Millisecond

// waitForUsbipStatusCleared blocks until usbip_status leaves the "used"
// state, the device disappears, the bounded timeout fires, or ctx is
// cancelled. Writing -1 to usbip_sockfd only schedules the kernel-side down
// event; without this wait the broadcast that follows ReleaseImport would
// re-read the still-"used" status and emit no delta, leaving subscribers
// stuck on the busy view.
func waitForUsbipStatusCleared(ctx context.Context, busid string) {
	deadline := time.Now().Add(finishImportStatusTimeout)
	for {
		status, err := readUsbipStatus(busid)
		if err != nil || status != usbipStatusUsed {
			return
		}
		if !time.Now().Before(deadline) {
			return
		}
		if !sleepCtx(ctx, finishImportStatusPollInterval) {
			return
		}
	}
}

// readPrimaryVHCIStatus reads every status* file under
// /sys/devices/platform/vhci_hcd.0 and concatenates the rows in lexical
// order. status reports controller 0; status.N reports controller N.
// Port numbers are already globally unique — no remapping is needed.
func readPrimaryVHCIStatus() ([]vhciStatusRecord, error) {
	matches, err := filepath.Glob(filepath.Join(sysVHCIControllerV0, "status*"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	records := make([]vhciStatusRecord, 0)
	for _, path := range matches {
		secondary, parseErr := vhciSecondaryFromStatusFile(filepath.Base(path))
		if parseErr != nil {
			continue
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, readErr
		}
		records = append(records, parseVHCIStatus(secondary, string(raw))...)
	}
	return records, nil
}

func readAllVHCIStatus() []vhciStatusRecord {
	records, err := readPrimaryVHCIStatus()
	if err != nil {
		return nil
	}
	return records
}

func vhciPickFreePort(speed uint32, skip map[int]struct{}) (int, error) {
	targetHub := "hs"
	switch speed {
	case SpeedSuper, SpeedSuperPlus:
		targetHub = "ss"
	}
	records, err := readPrimaryVHCIStatus()
	if err != nil {
		return -1, err
	}
	for _, record := range records {
		if record.hub != targetHub || record.state != 4 {
			continue
		}
		_, skipped := skip[record.port]
		if skipped {
			continue
		}
		return record.port, nil
	}
	return -1, E.New("no free ", targetHub, " vhci port")
}

func vhciSecondaryFromStatusFile(name string) (int, error) {
	if name == "status" {
		return 0, nil
	}
	suffix := strings.TrimPrefix(name, "status.")
	if suffix == name {
		return 0, E.New("not a status file: ", name)
	}
	return strconv.Atoi(suffix)
}

func parseVHCIStatus(secondary int, raw string) []vhciStatusRecord {
	scanner := bufio.NewScanner(strings.NewReader(raw))
	records := make([]vhciStatusRecord, 0)
	first := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if first {
			first = false
			continue
		}
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		port, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		state, err := strconv.Atoi(fields[2])
		if err != nil {
			continue
		}
		records = append(records, vhciStatusRecord{
			secondary: secondary,
			hub:       fields[0],
			port:      port,
			state:     state,
		})
	}
	return records
}

func ensureKernelPath(path string, module string, description string) error {
	_, err := os.Stat(path)
	if err == nil {
		return nil
	}
	if os.Getuid() != 0 {
		return E.Cause(err, description, " not present; root is required to load kernel module ", module)
	}
	modprobePath, modprobeErr := findModprobePath()
	if modprobeErr != nil {
		return E.Cause(modprobeErr, "load kernel module ", module, " for ", description)
	}
	output, modprobeErr := shell.Exec(modprobePath, module).Read()
	if modprobeErr != nil {
		return E.Extend(E.Cause(modprobeErr, "load kernel module ", module, " for ", description), strings.TrimSpace(output))
	}
	if _, err = os.Stat(path); err != nil {
		return E.Cause(err, description, " still not present after loading kernel module ", module)
	}
	return nil
}

func findModprobePath() (string, error) {
	if path, err := exec.LookPath("modprobe"); err == nil {
		return path, nil
	}
	for _, path := range []string{"/usr/sbin/modprobe", "/sbin/modprobe", "/usr/bin/modprobe", "/bin/modprobe"} {
		info, err := os.Stat(path)
		if err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
			return path, nil
		}
	}
	return "", E.New("modprobe executable not found")
}

func writeSysfs(path, content string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}

func readString(dir, attr string) string {
	raw, err := os.ReadFile(filepath.Join(dir, attr))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func readHexU16(dir, attr string) (uint16, error) {
	s := readString(dir, attr)
	if s == "" {
		return 0, E.New(attr, " missing")
	}
	v, err := strconv.ParseUint(s, 16, 16)
	if err != nil {
		return 0, err
	}
	return uint16(v), nil
}

func readHexU8(dir, attr string) (uint8, error) {
	s := readString(dir, attr)
	if s == "" {
		return 0, E.New(attr, " missing")
	}
	v, err := strconv.ParseUint(s, 16, 8)
	if err != nil {
		return 0, err
	}
	return uint8(v), nil
}

func readDecU8(dir, attr string) (uint8, error) {
	s := readString(dir, attr)
	if s == "" {
		return 0, E.New(attr, " missing")
	}
	v, err := strconv.ParseUint(s, 10, 8)
	if err != nil {
		return 0, err
	}
	return uint8(v), nil
}

func readDecU32(dir, attr string) (uint32, error) {
	s := readString(dir, attr)
	if s == "" {
		return 0, E.New(attr, " missing")
	}
	v, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(v), nil
}

func speedCodeFromString(s string) uint32 {
	switch s {
	case "1.5":
		return SpeedLow
	case "12":
		return SpeedFull
	case "480":
		return SpeedHigh
	case "5000":
		return SpeedSuper
	case "10000", "20000":
		return SpeedSuperPlus
	default:
		return SpeedUnknown
	}
}
