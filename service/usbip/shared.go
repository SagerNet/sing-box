//go:build linux || (darwin && cgo)

package usbip

import (
	"context"
	"fmt"
	"net"
	"strings"
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

func assignMatchedBusIDsWithRetained(
	targets []clientTarget,
	current []string,
	entries []DeviceEntry,
	knownKeys map[string]DeviceKey,
	activeCurrent map[string]struct{},
) []string {
	if len(targets) == 0 {
		return nil
	}
	keysByBusID := make(map[string]DeviceKey, len(entries))
	for i := range entries {
		busid := entries[i].Info.BusIDString()
		if busid == "" {
			continue
		}
		keysByBusID[busid] = entryDeviceKey(entries[i])
	}
	currentKey := func(busid string) (DeviceKey, bool) {
		if key, ok := keysByBusID[busid]; ok {
			return key, true
		}
		if _, active := activeCurrent[busid]; !active {
			return DeviceKey{}, false
		}
		key, ok := knownKeys[busid]
		return key, ok
	}
	nextAssigned := make([]string, len(targets))
	reserved := make(map[string]struct{}, len(targets))
	for i, target := range targets {
		if target.fixedBusID == "" {
			continue
		}
		if _, ok := keysByBusID[target.fixedBusID]; ok {
			nextAssigned[i] = target.fixedBusID
			reserved[target.fixedBusID] = struct{}{}
			continue
		}
		if i >= len(current) || current[i] != target.fixedBusID {
			continue
		}
		if _, ok := currentKey(target.fixedBusID); ok {
			nextAssigned[i] = target.fixedBusID
			reserved[target.fixedBusID] = struct{}{}
		}
	}
	for i, target := range targets {
		if target.fixedBusID != "" || i >= len(current) || current[i] == "" {
			continue
		}
		if _, ok := reserved[current[i]]; ok {
			continue
		}
		key, ok := currentKey(current[i])
		if !ok || !matches(target.match, key) {
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
		key := entryDeviceKey(entries[i])
		if _, claimed := reserved[key.BusID]; claimed {
			continue
		}
		if matches(match, key) {
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
		parts = append(parts, fmt.Sprintf("vendor_id=0x%04x", uint16(m.VendorID)))
	}
	if m.ProductID != 0 {
		parts = append(parts, fmt.Sprintf("product_id=0x%04x", uint16(m.ProductID)))
	}
	if m.Serial != "" {
		parts = append(parts, "serial="+m.Serial)
	}
	return "{" + strings.Join(parts, ",") + "}"
}
