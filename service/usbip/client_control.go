//go:build linux || (darwin && cgo) || windows

package usbip

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
