//go:build linux || (darwin && cgo) || windows

package usbip

import (
	"context"
)

func (c *ClientService) applyControlDelta(delta controlDeviceDelta) {
	c.remoteAccess.Lock()
	if c.remoteDevicesV2 == nil {
		c.remoteDevicesV2 = make(map[string]DeviceInfoV2)
	}
	for _, busid := range delta.Removed {
		delete(c.remoteDevicesV2, busid)
	}
	for _, device := range delta.Added {
		if device.BusID == "" {
			continue
		}
		c.remoteDevicesV2[device.BusID] = device
	}
	for _, device := range delta.Updated {
		if device.BusID == "" {
			continue
		}
		c.remoteDevicesV2[device.BusID] = device
	}
	values := sortedDeviceInfoV2Values(c.remoteDevicesV2)
	c.remoteAccess.Unlock()
	c.applyRemoteDeviceState(values)
}

func (c *ClientService) syncRemoteStateAndResetControlState(ctx context.Context) error {
	entries, err := c.fetchDevList(ctx)
	if err != nil {
		return err
	}
	devices := make(map[string]DeviceInfoV2, len(entries))
	for _, entry := range entries {
		device := deviceInfoV2FromEntry(entry, "", "", deviceStateAvailable, 0, deviceStateAvailable)
		if device.BusID == "" {
			continue
		}
		devices[device.BusID] = device
	}
	c.remoteAccess.Lock()
	c.remoteDevicesV2 = devices
	c.remoteAccess.Unlock()
	if !c.assignment.Matched() {
		c.applyRemoteExports(entries)
		return nil
	}
	c.applyMatchedExportsWithRetained(entries, nil)
	return nil
}
