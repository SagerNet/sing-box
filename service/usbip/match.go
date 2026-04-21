package usbip

import (
	"github.com/sagernet/sing-box/option"
)

// DeviceKey is the minimal set of fields needed to evaluate a USBIPDeviceMatch.
// Both the local sysfs enumerator and the remote OP_REP_DEVLIST parser populate
// this before running matches.
type DeviceKey struct {
	BusID     string
	VendorID  uint16
	ProductID uint16
	Serial    string
}

// Matches reports whether d satisfies m. Non-zero fields AND together; an
// all-zero match is treated as non-matching (callers should reject such
// configs earlier).
func Matches(m option.USBIPDeviceMatch, d DeviceKey) bool {
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
