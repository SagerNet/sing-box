//go:build linux

package usbip

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

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

// sysfsDevice captures the subset of USB device attributes needed for
// matching, export, and OP_REP_DEVLIST emission.
type sysfsDevice struct {
	BusID          string
	Path           string // sysfs path, e.g. /sys/bus/usb/devices/1-2
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

func (d *sysfsDevice) key() DeviceKey {
	return DeviceKey{
		BusID:     d.BusID,
		VendorID:  d.VendorID,
		ProductID: d.ProductID,
		Serial:    d.Serial,
	}
}

func (d *sysfsDevice) toProtocol() DeviceInfoTruncated {
	var info DeviceInfoTruncated
	encodePathField(&info.Path, d.Path)
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

func (d *sysfsDevice) toDeviceEntry() DeviceEntry {
	return DeviceEntry{
		Info:       d.toProtocol(),
		Interfaces: d.Interfaces,
		Serial:     d.Serial,
	}
}

type vhciStatusRecord struct {
	hub   string
	port  int
	state int
}

// ensureHostDriver verifies the usbip-host kernel driver is loaded.
func ensureHostDriver() error {
	return ensureKernelPath(sysUsbipHostDriver, "usbip-host", "usbip-host driver")
}

// ensureVHCI verifies the vhci_hcd controller is loaded.
func ensureVHCI() error {
	return ensureKernelPath(sysVHCIControllerV0, "vhci-hcd", "vhci_hcd.0")
}

// listUSBDevices enumerates /sys/bus/usb/devices, returning non-interface
// device entries that expose idVendor.
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

// readSysfsDevice populates a sysfsDevice from the attributes at path.
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

// readInterfaces reads the per-interface descriptors sibling to the device node.
func readInterfaces(devicePath, busid string, configValue uint8, count int) []DeviceInterface {
	if count == 0 {
		return nil
	}
	interfaces := make([]DeviceInterface, count)
	for i := 0; i < count; i++ {
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

// currentDriver returns the driver currently bound to busid, or "" if none.
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

// unbindFromDriver detaches busid from driver.
func unbindFromDriver(busid, driver string) error {
	path := filepath.Join("/sys/bus/usb/drivers", driver, "unbind")
	return writeSysfs(path, busid)
}

// bindToDriver attaches busid to driver.
func bindToDriver(busid, driver string) error {
	path := filepath.Join("/sys/bus/usb/drivers", driver, "bind")
	return writeSysfs(path, busid)
}

// hostMatchBusID writes "add <busid>" / "del <busid>" to the usbip-host
// match_busid attribute. Returns nil on ENOENT for "del" (idempotent).
func hostMatchBusID(busid string, add bool) error {
	verb := "del"
	if add {
		verb = "add"
	}
	path := filepath.Join(sysUsbipHostDriver, "match_busid")
	return writeSysfs(path, verb+" "+busid)
}

// hostBind attaches busid to usbip-host.
func hostBind(busid string) error {
	return writeSysfs(filepath.Join(sysUsbipHostDriver, "bind"), busid)
}

// hostUnbind detaches busid from usbip-host.
func hostUnbind(busid string) error {
	return writeSysfs(filepath.Join(sysUsbipHostDriver, "unbind"), busid)
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
	return ensureHostDriver()
}

// readUsbipStatus returns the usbip_status attribute value for busid.
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

// writeUsbipSockfd hands the fd for busid to the usbip-host kernel driver.
// Passing -1 as fd releases the connection.
func writeUsbipSockfd(busid string, fd int) error {
	return writeSysfs(filepath.Join(sysBusUSBDevices, busid, "usbip_sockfd"), strconv.Itoa(fd))
}

// vhciPickFreePort scans the vhci_hcd.0 status table and returns a free port
// from the hub that matches the remote device speed.
func vhciPickFreePort(speed uint32) (int, error) {
	records, err := readVHCIStatus()
	if err != nil {
		return -1, err
	}
	targetHub := vhciHubForSpeed(speed)
	for _, record := range records {
		if record.hub != targetHub || record.state != 4 {
			continue
		}
		return record.port, nil
	}
	return -1, E.New("no free ", targetHub, " vhci port")
}

// vhciPortUsed reports whether the given port is currently in VDEV_ST_USED (6).
func vhciPortUsed(port int) (bool, error) {
	records, err := readVHCIStatus()
	if err != nil {
		return false, err
	}
	for _, record := range records {
		if record.port != port {
			continue
		}
		return record.state == 6, nil // VDEV_ST_USED
	}
	return false, nil
}

// vhciAttach writes "port fd devid speed" to the attach attribute.
func vhciAttach(port int, fd uintptr, devid uint32, speed uint32) error {
	line := fmt.Sprintf("%d %d %d %d", port, int(fd), devid, speed)
	return writeSysfs(filepath.Join(sysVHCIControllerV0, "attach"), line)
}

// vhciDetach writes the port number to the detach attribute.
func vhciDetach(port int) error {
	return writeSysfs(filepath.Join(sysVHCIControllerV0, "detach"), strconv.Itoa(port))
}

func readVHCIStatus() ([]vhciStatusRecord, error) {
	raw, err := os.ReadFile(filepath.Join(sysVHCIControllerV0, "status"))
	if err != nil {
		return nil, err
	}
	return parseVHCIStatus(string(raw)), nil
}

func parseVHCIStatus(raw string) []vhciStatusRecord {
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
			hub:   fields[0],
			port:  port,
			state: state,
		})
	}
	return records
}

func vhciHubForSpeed(speed uint32) string {
	switch speed {
	case SpeedSuper, SpeedSuperPlus:
		return "ss"
	default:
		return "hs"
	}
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

// --- small helpers ------------------------------------------------------

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

// speedCodeFromString maps the sysfs "speed" attribute to the USB/IP wire
// enum usb_device_speed.
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
