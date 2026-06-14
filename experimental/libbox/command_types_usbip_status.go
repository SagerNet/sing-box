package libbox

import "github.com/sagernet/sing-box/daemon"

type USBIPServerStatusUpdate struct {
	servers []*USBIPServerStatus
}

func (u *USBIPServerStatusUpdate) Servers() USBIPServerStatusIterator {
	return newIterator(u.servers)
}

type USBIPServerStatusIterator interface {
	Next() *USBIPServerStatus
	HasNext() bool
}

type USBIPServerStatus struct {
	ServerTag string
	devices   []*USBSharedDevice
}

func (s *USBIPServerStatus) Devices() USBSharedDeviceIterator {
	return newIterator(s.devices)
}

type USBSharedDeviceIterator interface {
	Next() *USBSharedDevice
	HasNext() bool
}

const (
	USBDeviceStateIdle int32 = iota
	USBDeviceStateAttached
	USBDeviceStateUnavailable
)

const (
	USBBackendUnspecified int32 = iota
	USBBackendLinuxSysfs
	USBBackendDynamic
	USBBackendDarwinIOKit
	USBBackendWindowsVBoxUSB
)

type USBSharedDevice struct {
	BusID              string
	StableID           string
	Backend            int32
	State              int32
	DeviceID           string
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
	interfaces         []*USBSharedDeviceInterface
}

func (d *USBSharedDevice) Interfaces() USBSharedDeviceInterfaceIterator {
	return newIterator(d.interfaces)
}

type USBSharedDeviceInterfaceIterator interface {
	Next() *USBSharedDeviceInterface
	HasNext() bool
}

type USBSharedDeviceInterface struct {
	InterfaceClass    int32
	InterfaceSubClass int32
	InterfaceProtocol int32
}

type USBIPServerStatusHandler interface {
	OnStatusUpdate(status *USBIPServerStatusUpdate)
	OnError(message string)
}

type USBIPServerStatusSubscription struct {
	streamSession
}

func usbipServerStatusUpdateFromGRPC(update *daemon.USBIPServerStatusUpdate) *USBIPServerStatusUpdate {
	servers := make([]*USBIPServerStatus, len(update.Servers))
	for i, server := range update.Servers {
		servers[i] = usbipServerStatusFromGRPC(server)
	}
	return &USBIPServerStatusUpdate{servers: servers}
}

func usbipServerStatusFromGRPC(status *daemon.USBIPServerStatus) *USBIPServerStatus {
	devices := make([]*USBSharedDevice, len(status.Devices))
	for i, device := range status.Devices {
		devices[i] = usbSharedDeviceFromGRPC(device)
	}
	return &USBIPServerStatus{
		ServerTag: status.GetServerTag(),
		devices:   devices,
	}
}

func usbSharedDeviceFromGRPC(device *daemon.USBSharedDevice) *USBSharedDevice {
	descriptor := device.GetDescriptor_()
	interfaces := make([]*USBSharedDeviceInterface, len(descriptor.GetInterfaces()))
	for i, deviceInterface := range descriptor.GetInterfaces() {
		interfaces[i] = &USBSharedDeviceInterface{
			InterfaceClass:    int32(deviceInterface.GetInterfaceClass()),
			InterfaceSubClass: int32(deviceInterface.GetInterfaceSubClass()),
			InterfaceProtocol: int32(deviceInterface.GetInterfaceProtocol()),
		}
	}
	return &USBSharedDevice{
		BusID:              device.GetBusId(),
		StableID:           device.GetStableId(),
		Backend:            int32(device.GetBackend()),
		State:              int32(device.GetState()),
		DeviceID:           descriptor.GetDeviceId(),
		BusNum:             int32(descriptor.GetBusNum()),
		DevNum:             int32(descriptor.GetDevNum()),
		Speed:              int32(descriptor.GetSpeed()),
		VendorID:           int32(descriptor.GetVendorId()),
		ProductID:          int32(descriptor.GetProductId()),
		BCDDevice:          int32(descriptor.GetBcdDevice()),
		DeviceClass:        int32(descriptor.GetDeviceClass()),
		DeviceSubClass:     int32(descriptor.GetDeviceSubClass()),
		DeviceProtocol:     int32(descriptor.GetDeviceProtocol()),
		ConfigurationValue: int32(descriptor.GetConfigurationValue()),
		NumConfigurations:  int32(descriptor.GetNumConfigurations()),
		Serial:             descriptor.GetSerial(),
		Product:            descriptor.GetProduct(),
		interfaces:         interfaces,
	}
}
