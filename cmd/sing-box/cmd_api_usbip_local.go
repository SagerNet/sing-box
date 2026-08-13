//go:build with_usbip && (linux || (darwin && cgo) || windows)

package main

import (
	"cmp"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/sagernet/sing-usbip"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
)

func runAPIUsbipDeviceList() error {
	devices, err := listLocalUSBDevices()
	if err != nil {
		return err
	}
	table := tableWriter{
		header:       []string{"BUSID", "VID:PID", "PRODUCT", "SERIAL"},
		emptyMessage: "no local USB devices",
	}
	for _, device := range devices {
		table.addRow(
			device.Entry.Info.BusIDString(),
			usbipVendorProduct(device.Entry.Info.IDVendor, device.Entry.Info.IDProduct),
			device.Entry.Product,
			device.Entry.Serial,
		)
	}
	table.flush()
	return nil
}

func runAPIUsbipDeviceShow(selector string) error {
	devices, err := listLocalUSBDevices()
	if err != nil {
		return err
	}
	device, err := resolveLocalUSBDevice(devices, selector)
	if err != nil {
		return err
	}
	info := device.Entry.Info
	var block blockWriter
	block.addLine("busid", info.BusIDString())
	block.addLine("stable id", device.StableID)
	block.addLine("backend", device.Backend.String())
	if device.Entry.Product != "" {
		block.addLine("product", device.Entry.Product)
	}
	if device.Entry.Serial != "" {
		block.addLine("serial", device.Entry.Serial)
	}
	block.addLine("vendor id", fmt.Sprintf("%04x", info.IDVendor))
	block.addLine("product id", fmt.Sprintf("%04x", info.IDProduct))
	block.addLine("device version", fmt.Sprintf("%x.%02x", info.BCDDevice>>8, info.BCDDevice&0xff))
	block.addLine("bus", F.ToString(info.BusNum))
	block.addLine("device", F.ToString(info.DevNum))
	block.addLine("speed", usbipSpeedString(info.Speed))
	block.addLine("device class", usbipDeviceClassString(info.BDeviceClass))
	block.addLine("device subclass", fmt.Sprintf("%02x", info.BDeviceSubClass))
	block.addLine("device protocol", fmt.Sprintf("%02x", info.BDeviceProtocol))
	if info.BNumConfigurations > 0 {
		block.addLine("configurations", F.ToString(info.BNumConfigurations, " (active #", info.BConfigurationValue, ")"))
	}
	for index, deviceInterface := range device.Entry.Interfaces {
		block.addLine(F.ToString("interface ", index), fmt.Sprintf(
			"class %02x subclass %02x protocol %02x",
			deviceInterface.BInterfaceClass,
			deviceInterface.BInterfaceSubClass,
			deviceInterface.BInterfaceProtocol,
		))
	}
	block.flush()
	return nil
}

// ListLocalDevices filters hubs on linux and darwin but not on windows.
func listLocalUSBDevices() ([]usbip.LocalDeviceInfo, error) {
	devices, err := usbip.ListLocalDevices()
	if err != nil {
		return nil, err
	}
	devices = common.Filter(devices, func(it usbip.LocalDeviceInfo) bool {
		return it.Entry.Info.BDeviceClass != 0x09
	})
	slices.SortFunc(devices, func(a usbip.LocalDeviceInfo, b usbip.LocalDeviceInfo) int {
		return compareBusID(a.Entry.Info.BusIDString(), b.Entry.Info.BusIDString())
	})
	return devices, nil
}

func resolveLocalUSBDevice(devices []usbip.LocalDeviceInfo, selector string) (usbip.LocalDeviceInfo, error) {
	index := slices.IndexFunc(devices, func(it usbip.LocalDeviceInfo) bool {
		return it.Entry.Info.BusIDString() == selector
	})
	if index != -1 {
		return devices[index], nil
	}
	vendorID, productID, serial, valid := parseLocalUSBDeviceSelector(selector)
	if !valid {
		return usbip.LocalDeviceInfo{}, E.New("no local USB device matches ", selector)
	}
	matches := common.Filter(devices, func(it usbip.LocalDeviceInfo) bool {
		if it.Entry.Info.IDVendor != vendorID || it.Entry.Info.IDProduct != productID {
			return false
		}
		return serial == "" || it.Entry.Serial == serial
	})
	switch len(matches) {
	case 0:
		return usbip.LocalDeviceInfo{}, E.New("no local USB device matches ", selector)
	case 1:
		return matches[0], nil
	}
	writeLocalUSBDeviceMatches(matches)
	return usbip.LocalDeviceInfo{}, E.New(selector, " matches ", len(matches), " devices")
}

func parseLocalUSBDeviceSelector(selector string) (vendorID uint16, productID uint16, serial string, valid bool) {
	parts := strings.SplitN(selector, ":", 3)
	if len(parts) < 2 {
		return 0, 0, "", false
	}
	vendorID, valid = parseUSBIdentifier(parts[0])
	if !valid {
		return 0, 0, "", false
	}
	productID, valid = parseUSBIdentifier(parts[1])
	if !valid {
		return 0, 0, "", false
	}
	if len(parts) == 3 {
		serial = parts[2]
		if serial == "" {
			return 0, 0, "", false
		}
	}
	return vendorID, productID, serial, true
}

func parseUSBIdentifier(value string) (uint16, bool) {
	if len(value) != 4 {
		return 0, false
	}
	parsed, err := strconv.ParseUint(value, 16, 16)
	if err != nil {
		return 0, false
	}
	return uint16(parsed), true
}

func writeLocalUSBDeviceMatches(matches []usbip.LocalDeviceInfo) {
	busIDWidth := 0
	productWidth := 0
	for _, device := range matches {
		busIDWidth = max(busIDWidth, len(device.Entry.Info.BusIDString()))
		productWidth = max(productWidth, len(device.Entry.Product))
	}
	var output strings.Builder
	for _, device := range matches {
		output.WriteString("  ")
		output.WriteString(padUSBIPCell(device.Entry.Info.BusIDString(), busIDWidth))
		output.WriteString("  ")
		output.WriteString(usbipVendorProduct(device.Entry.Info.IDVendor, device.Entry.Info.IDProduct))
		if device.Entry.Product != "" || device.Entry.Serial != "" {
			output.WriteString("  ")
			output.WriteString(padUSBIPCell(device.Entry.Product, productWidth))
		}
		if device.Entry.Serial != "" {
			output.WriteString("  (serial ")
			output.WriteString(device.Entry.Serial)
			output.WriteString(")")
		}
		output.WriteString("\n")
	}
	os.Stderr.WriteString(output.String())
}

func padUSBIPCell(value string, width int) string {
	if len(value) >= width {
		return value
	}
	return value + strings.Repeat(" ", width-len(value))
}

func compareBusID(a string, b string) int {
	for len(a) > 0 && len(b) > 0 {
		aDigits := usbipDigitPrefix(a)
		bDigits := usbipDigitPrefix(b)
		if aDigits > 0 && bDigits > 0 {
			aValue, aErr := strconv.ParseUint(a[:aDigits], 10, 64)
			bValue, bErr := strconv.ParseUint(b[:bDigits], 10, 64)
			if aErr == nil && bErr == nil {
				if aValue != bValue {
					return cmp.Compare(aValue, bValue)
				}
				a = a[aDigits:]
				b = b[bDigits:]
				continue
			}
		}
		if a[0] != b[0] {
			return cmp.Compare(a[0], b[0])
		}
		a = a[1:]
		b = b[1:]
	}
	return cmp.Compare(len(a), len(b))
}

func usbipDigitPrefix(value string) int {
	index := 0
	for index < len(value) && value[index] >= '0' && value[index] <= '9' {
		index++
	}
	return index
}

func usbipSpeedString(speed uint32) string {
	switch speed {
	case usbip.SpeedLow:
		return "low (1.5 Mbps)"
	case usbip.SpeedFull:
		return "full (12 Mbps)"
	case usbip.SpeedHigh:
		return "high (480 Mbps)"
	case usbip.SpeedSuper:
		return "super (5 Gbps)"
	case usbip.SpeedSuperPlus:
		return "super+ (10 Gbps)"
	default:
		return F.ToString(speed)
	}
}

var usbipDeviceClassNames = map[uint8]string{
	0x00: "defined at interface level",
	0x01: "audio",
	0x02: "communications",
	0x03: "human interface device",
	0x05: "physical",
	0x06: "image",
	0x07: "printer",
	0x08: "mass storage",
	0x09: "hub",
	0x0a: "cdc data",
	0x0b: "smart card",
	0x0d: "content security",
	0x0e: "video",
	0x0f: "personal healthcare",
	0x10: "audio/video",
	0x11: "billboard",
	0x12: "usb type-c bridge",
	0xdc: "diagnostic",
	0xe0: "wireless controller",
	0xef: "miscellaneous",
	0xfe: "application specific",
	0xff: "vendor specific",
}

func usbipDeviceClassString(deviceClass uint8) string {
	name, found := usbipDeviceClassNames[deviceClass]
	if !found {
		return fmt.Sprintf("%02x", deviceClass)
	}
	return fmt.Sprintf("%02x (%s)", deviceClass, name)
}
