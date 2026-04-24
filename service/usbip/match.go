package usbip

import (
	"github.com/sagernet/sing-box/option"
)

type DeviceKey struct {
	BusID     string
	VendorID  uint16
	ProductID uint16
	Serial    string
}

func matches(m option.USBIPDeviceMatch, d DeviceKey) bool {
	if m.IsZero() {
		return false
	}
	if m.BusID != "" && m.BusID != d.BusID {
		return false
	}
	if m.VendorID != 0 && uint16(m.VendorID) != d.VendorID {
		return false
	}
	if m.ProductID != 0 && uint16(m.ProductID) != d.ProductID {
		return false
	}
	if m.Serial != "" && m.Serial != d.Serial {
		return false
	}
	return true
}
