package libbox

import (
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/common/x/list"
)

var (
	_ tun.DefaultInterfaceMonitor = (*platformDefaultInterfaceMonitor)(nil)
	_ InterfaceUpdateListener     = (*platformDefaultInterfaceMonitor)(nil)
)

type platformDefaultInterfaceMonitor struct {
	*platformInterfaceWrapper
	logger      logger.Logger
	element     *list.Element[tun.NetworkUpdateCallback]
	callbacks   list.List[tun.DefaultInterfaceUpdateCallback]
	myInterface string
}

func (m *platformDefaultInterfaceMonitor) Start() error {
	return m.iif.StartDefaultInterfaceMonitor(m)
}

func (m *platformDefaultInterfaceMonitor) Close() error {
	return m.iif.CloseDefaultInterfaceMonitor(m)
}

func (m *platformDefaultInterfaceMonitor) DefaultInterface() *control.Interface {
	m.defaultInterfaceAccess.Lock()
	defer m.defaultInterfaceAccess.Unlock()
	return m.defaultInterface
}

func (m *platformDefaultInterfaceMonitor) OverrideAndroidVPN() bool {
	return false
}

func (m *platformDefaultInterfaceMonitor) AndroidVPNEnabled() bool {
	return false
}

func (m *platformDefaultInterfaceMonitor) RegisterCallback(callback tun.DefaultInterfaceUpdateCallback) *list.Element[tun.DefaultInterfaceUpdateCallback] {
	m.defaultInterfaceAccess.Lock()
	defer m.defaultInterfaceAccess.Unlock()
	return m.callbacks.PushBack(callback)
}

func (m *platformDefaultInterfaceMonitor) UnregisterCallback(element *list.Element[tun.DefaultInterfaceUpdateCallback]) {
	m.defaultInterfaceAccess.Lock()
	defer m.defaultInterfaceAccess.Unlock()
	m.callbacks.Remove(element)
}

func (m *platformDefaultInterfaceMonitor) UpdateDefaultInterface(interfaceName string, interfaceIndex32 int32, isExpensive bool, isConstrained bool) {
	if sFixAndroidStack {
		done := make(chan struct{})
		go func() {
			m.updateDefaultInterface(interfaceName, interfaceIndex32, isExpensive, isConstrained)
			close(done)
		}()
		<-done
	} else {
		m.updateDefaultInterface(interfaceName, interfaceIndex32, isExpensive, isConstrained)
	}
}

func (m *platformDefaultInterfaceMonitor) updateDefaultInterface(interfaceName string, interfaceIndex32 int32, isExpensive bool, isConstrained bool) {
	m.isExpensive = isExpensive
	m.isConstrained = isConstrained
	err := m.networkManager.UpdateInterfaces()
	if err != nil {
		m.logger.Error(E.Cause(err, "update interfaces"))
	}
	m.defaultInterfaceAccess.Lock()
	if interfaceIndex32 == -1 {
		m.defaultInterface = nil
		callbacks := m.callbacks.Array()
		m.defaultInterfaceAccess.Unlock()
		for _, callback := range callbacks {
			callback(nil, 0)
		}
		return
	}
	oldInterface := m.defaultInterface
	newInterface, err := m.networkManager.InterfaceFinder().ByIndex(int(interfaceIndex32))
	if err != nil {
		m.defaultInterfaceAccess.Unlock()
		m.logger.Error(E.Cause(err, "find updated interface: ", interfaceName))
		return
	}
	m.defaultInterface = newInterface
	if oldInterface != nil && oldInterface.Name == m.defaultInterface.Name && oldInterface.Index == m.defaultInterface.Index {
		m.defaultInterfaceAccess.Unlock()
		return
	}
	callbacks := m.callbacks.Array()
	m.defaultInterfaceAccess.Unlock()
	for _, callback := range callbacks {
		callback(newInterface, 0)
	}
}

func (m *platformDefaultInterfaceMonitor) RegisterMyInterface(interfaceName string) {
	m.defaultInterfaceAccess.Lock()
	defer m.defaultInterfaceAccess.Unlock()
	m.myInterface = interfaceName
}

func (m *platformDefaultInterfaceMonitor) MyInterface() string {
	m.defaultInterfaceAccess.Lock()
	defer m.defaultInterfaceAccess.Unlock()
	return m.myInterface
}
