//go:build linux || (darwin && cgo)

package usbip

import (
	"slices"

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

// SelectMatches returns the indexes of keys that match at least one
// non-zero pattern. Indexes are deduplicated and returned in ascending
// order so callers iterate devices in a stable sequence.
func SelectMatches(patterns []option.USBIPDeviceMatch, keys []DeviceKey) []int {
	if len(patterns) == 0 || len(keys) == 0 {
		return nil
	}
	hit := make(map[int]struct{})
	for _, pattern := range patterns {
		for i := range keys {
			if matches(pattern, keys[i]) {
				hit[i] = struct{}{}
			}
		}
	}
	out := make([]int, 0, len(hit))
	for i := range hit {
		out = append(out, i)
	}
	slices.Sort(out)
	return out
}
