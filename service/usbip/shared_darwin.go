//go:build darwin && cgo

package usbip

import (
	"context"
	"net"
	"time"

	"github.com/sagernet/sing-box/option"
)

type clientTarget struct {
	fixedBusID string
	match      option.USBIPDeviceMatch
}

func (t clientTarget) description() string {
	if t.fixedBusID != "" {
		return describeMatch(option.USBIPDeviceMatch{BusID: t.fixedBusID})
	}
	return describeMatch(t.match)
}

func isBusIDOnlyMatch(m option.USBIPDeviceMatch) bool {
	return m.BusID != "" && m.VendorID == 0 && m.ProductID == 0 && m.Serial == ""
}

func assignMatchedBusIDs(targets []clientTarget, current []string, entries []DeviceEntry) []string {
	if len(targets) == 0 {
		return nil
	}
	keysByBusID := make(map[string]DeviceKey, len(entries))
	for i := range entries {
		busid := entries[i].Info.BusIDString()
		if busid == "" {
			continue
		}
		keysByBusID[busid] = DeviceKey{
			BusID:     busid,
			VendorID:  entries[i].Info.IDVendor,
			ProductID: entries[i].Info.IDProduct,
			Serial:    entries[i].Info.SerialString(),
		}
	}
	nextAssigned := make([]string, len(targets))
	reserved := make(map[string]struct{}, len(targets))
	for i, target := range targets {
		if target.fixedBusID == "" {
			continue
		}
		if _, ok := keysByBusID[target.fixedBusID]; !ok {
			continue
		}
		nextAssigned[i] = target.fixedBusID
		reserved[target.fixedBusID] = struct{}{}
	}
	for i, target := range targets {
		if target.fixedBusID != "" || i >= len(current) || current[i] == "" {
			continue
		}
		if _, ok := reserved[current[i]]; ok {
			continue
		}
		key, ok := keysByBusID[current[i]]
		if !ok || !Matches(target.match, key) {
			continue
		}
		nextAssigned[i] = current[i]
		reserved[current[i]] = struct{}{}
	}
	for i, target := range targets {
		if target.fixedBusID != "" || nextAssigned[i] != "" {
			continue
		}
		nextAssigned[i] = firstMatchingUnclaimedBusID(target.match, entries, reserved)
		if nextAssigned[i] != "" {
			reserved[nextAssigned[i]] = struct{}{}
		}
	}
	return nextAssigned
}

func firstMatchingUnclaimedBusID(match option.USBIPDeviceMatch, entries []DeviceEntry, reserved map[string]struct{}) string {
	for i := range entries {
		key := DeviceKey{
			BusID:     entries[i].Info.BusIDString(),
			VendorID:  entries[i].Info.IDVendor,
			ProductID: entries[i].Info.IDProduct,
			Serial:    entries[i].Info.SerialString(),
		}
		if _, claimed := reserved[key.BusID]; claimed {
			continue
		}
		if Matches(match, key) {
			return key.BusID
		}
	}
	return ""
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func closeConnOnContextDone(ctx context.Context, conn net.Conn) func() {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()
	return func() {
		close(done)
	}
}

func describeMatch(m option.USBIPDeviceMatch) string {
	var parts []string
	if m.BusID != "" {
		parts = append(parts, "busid="+m.BusID)
	}
	if m.VendorID != 0 {
		parts = append(parts, "vendor_id=0x"+hex16(uint16(m.VendorID)))
	}
	if m.ProductID != 0 {
		parts = append(parts, "product_id=0x"+hex16(uint16(m.ProductID)))
	}
	if m.Serial != "" {
		parts = append(parts, "serial="+m.Serial)
	}
	return "{" + joinComma(parts) + "}"
}

func hex8(v uint8) string {
	const hexdigits = "0123456789abcdef"
	return string([]byte{hexdigits[(v>>4)&0xf], hexdigits[v&0xf]})
}

func hex16(v uint16) string {
	const hexdigits = "0123456789abcdef"
	return string([]byte{
		hexdigits[(v>>12)&0xf],
		hexdigits[(v>>8)&0xf],
		hexdigits[(v>>4)&0xf],
		hexdigits[v&0xf],
	})
}

func hex32(v uint32) string {
	return hex16(uint16(v>>16)) + hex16(uint16(v))
}

func stringsCompare(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ","
		}
		out += p
	}
	return out
}
