//go:build with_usbip && (linux || (darwin && cgo) || windows)

package adapter

import (
	"context"

	"github.com/sagernet/sing-usbip"
)

type USBIPDynamicServer interface {
	AddDevice(info usbip.DynamicDeviceInfo, transport usbip.DeviceTransport) (string, error)
	RemoveDevice(busID string)
	SubscribeDevices(ctx context.Context, listener func([]usbip.ControlDeviceInfo))
}
