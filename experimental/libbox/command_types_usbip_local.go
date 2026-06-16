package libbox

type USBLocalDeviceInfo struct {
	StableID           string
	BusID              string
	Backend            int32
	BusNum             int32
	DevNum             int32
	Speed              int32
	VendorID           int32
	ProductID          int32
	BCDDevice          int32
	DeviceClass        int32
	DeviceSubClass     int32
	DeviceProtocol     int32
	ConfigurationValue int32
	NumConfigurations  int32
	Serial             string
	Product            string

	interfaces []*USBSharedDeviceInterface
}

func (d *USBLocalDeviceInfo) Interfaces() USBSharedDeviceInterfaceIterator {
	return newIterator(d.interfaces)
}

type USBLocalDeviceInfoIterator interface {
	Next() *USBLocalDeviceInfo
	HasNext() bool
}

type USBLocalProvidedDevice struct {
	ServerTag     string
	DeviceID      string
	LocalDeviceID string
	Label         string
	VendorID      int32
	ProductID     int32
}

type USBLocalProviderHandler interface {
	OnDeviceError(serverTag string, deviceID string, message string)
	OnSessionError(serverTag string, message string)
	OnLocalDevicesChanged()
}
