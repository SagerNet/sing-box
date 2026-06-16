//go:build !with_usbip || !darwin || ios || !cgo

package libbox

import "os"

type USBLocalProviderManager struct{}

func (c *CommandClient) NewUSBLocalProvider(handler USBLocalProviderHandler) (*USBLocalProviderManager, error) {
	return nil, os.ErrInvalid
}

func (m *USBLocalProviderManager) ListDevices() (USBLocalDeviceInfoIterator, error) {
	return nil, os.ErrInvalid
}

func (m *USBLocalProviderManager) Attach(serverTag string, localDeviceID string) (*USBLocalProvidedDevice, error) {
	return nil, os.ErrInvalid
}

func (m *USBLocalProviderManager) Detach(deviceID string) error {
	return os.ErrInvalid
}

func (m *USBLocalProviderManager) Close() error {
	return os.ErrInvalid
}
