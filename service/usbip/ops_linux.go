//go:build linux

package usbip

type usbEventListener interface {
	Close() error
	WaitUSBEvent() error
}

type usbipOps struct {
	ensureHostDriver func() error
	ensureVHCI       func() error

	listUSBDevices    func() ([]sysfsDevice, error)
	readSysfsDevice   func(busid string, path string) (sysfsDevice, error)
	currentDriver     func(busid string) (string, error)
	unbindFromDriver  func(busid string, driver string) error
	bindToDriver      func(busid string, driver string) error
	hostMatchBusID    func(busid string, add bool) error
	hostBind          func(busid string) error
	hostUnbind        func(busid string) error
	reloadHostDriver  func() error
	readUsbipStatus   func(busid string) (int, error)
	writeUsbipSockfd  func(busid string, fd int) error
	newUEventListener func() (usbEventListener, error)

	vhciPickFreePort func(speed uint32, skip map[int]struct{}) (int, error)
	vhciAttach       func(port int, fd uintptr, devid uint32, speed uint32) error
	vhciDetach       func(port int) error
}

var systemUSBIPOps = usbipOps{
	ensureHostDriver: ensureHostDriver,
	ensureVHCI:       ensureVHCI,
	listUSBDevices:   listUSBDevices,
	readSysfsDevice:  readSysfsDevice,
	currentDriver:    currentDriver,
	unbindFromDriver: unbindFromDriver,
	bindToDriver:     bindToDriver,
	hostMatchBusID:   hostMatchBusID,
	hostBind:         hostBind,
	hostUnbind:       hostUnbind,
	reloadHostDriver: reloadHostDriver,
	readUsbipStatus:  readUsbipStatus,
	writeUsbipSockfd: writeUsbipSockfd,
	newUEventListener: func() (usbEventListener, error) {
		return newUEventListener()
	},
	vhciPickFreePort: vhciPickFreePort,
	vhciAttach:       vhciAttach,
	vhciDetach:       vhciDetach,
}
