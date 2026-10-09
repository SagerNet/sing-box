package libbox

import (
	"net/netip"
	"slices"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/service/powerreport"
	tun "github.com/sagernet/sing-tun"
	"github.com/sagernet/sing-tun/dnsinfo"
	"github.com/sagernet/sing/common"
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
	logger                      logger.Logger
	callbacks                   list.List[tun.DefaultInterfaceUpdateCallback]
	myInterfaces                []string
	defaultInterfaceInitialized bool
	defaultDNSServers           []string
	dnsWatcher                  *dnsinfo.Watcher
	lastNetworkPath             string
}

func (m *platformDefaultInterfaceMonitor) Start() error {
	if C.IsDarwin {
		dnsWatcher, err := dnsinfo.NewWatcher(m.updateDNSServers, m.logger)
		if err != nil {
			return E.Cause(err, "watch DNS configuration")
		}
		m.dnsWatcher = dnsWatcher
	}
	err := m.iif.StartDefaultInterfaceMonitor(m)
	if err != nil {
		if C.IsDarwin {
			m.dnsWatcher.Close()
		}
		return err
	}
	return nil
}

func (m *platformDefaultInterfaceMonitor) Close() error {
	err := m.iif.CloseDefaultInterfaceMonitor(m)
	if C.IsDarwin {
		err = E.Errors(err, m.dnsWatcher.Close())
	}
	return err
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

func (m *platformDefaultInterfaceMonitor) UpdateNetworkPath(networkPath string) {
	if networkPath != m.lastNetworkPath {
		m.lastNetworkPath = networkPath
		m.logger.Debug("updated network path: ", networkPath)
	}
	if m.powerManager == nil {
		return
	}
	recorder := m.powerManager.Recorder()
	if recorder == nil {
		return
	}
	recorder.UpdateNetworkPath(networkPath)
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
	var recorder *powerreport.Recorder
	if m.powerManager != nil {
		recorder = m.powerManager.Recorder()
	}
	if recorder != nil {
		networkType := interfaceName
		if interfaceIndex32 == -1 {
			networkType = "none"
		} else {
			if isExpensive {
				networkType += ",expensive"
			}
			if isConstrained {
				networkType += ",constrained"
			}
		}
		recorder.UpdateNetworkType(networkType)
	}
	m.isExpensive = isExpensive
	m.isConstrained = isConstrained
	err := m.networkManager.UpdateInterfaces()
	if err != nil {
		m.logger.Error(E.Cause(err, "update interfaces"))
	}
	m.defaultInterfaceAccess.Lock()
	if interfaceIndex32 == -1 {
		m.defaultInterface = nil
		m.defaultDNSServers = nil
		m.defaultInterfaceInitialized = true
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
	oldDNSServers := m.defaultDNSServers
	m.defaultInterface = newInterface
	m.defaultDNSServers = m.readDNSServers(newInterface.Index)
	if m.defaultInterfaceInitialized && oldInterface != nil && oldInterface.Name == m.defaultInterface.Name && oldInterface.Index == m.defaultInterface.Index && slices.Equal(oldDNSServers, m.defaultDNSServers) {
		m.defaultInterfaceAccess.Unlock()
		return
	}
	m.defaultInterfaceInitialized = true
	callbacks := m.callbacks.Array()
	m.defaultInterfaceAccess.Unlock()
	for _, callback := range callbacks {
		callback(newInterface, 0)
	}
}

func (m *platformDefaultInterfaceMonitor) updateDNSServers() {
	m.defaultInterfaceAccess.Lock()
	defaultInterface := m.defaultInterface
	if defaultInterface == nil {
		m.defaultInterfaceAccess.Unlock()
		return
	}
	dnsServers := m.readDNSServers(defaultInterface.Index)
	if slices.Equal(m.defaultDNSServers, dnsServers) {
		m.defaultInterfaceAccess.Unlock()
		return
	}
	m.defaultDNSServers = dnsServers
	callbacks := m.callbacks.Array()
	m.defaultInterfaceAccess.Unlock()
	for _, callback := range callbacks {
		callback(defaultInterface, 0)
	}
}

func (m *platformDefaultInterfaceMonitor) readDNSServers(interfaceIndex int) []string {
	if C.IsDarwin {
		dnsConfiguration := dnsinfo.Copy()
		if dnsConfiguration == nil {
			return nil
		}
		return common.Map(dnsConfiguration.Select(interfaceIndex).Servers, func(it netip.AddrPort) string {
			return it.Addr().String()
		})
	}
	return common.Find(m.networkManager.NetworkInterfaces(), func(it adapter.NetworkInterface) bool {
		return it.Index == interfaceIndex
	}).DNSServers
}

func (m *platformDefaultInterfaceMonitor) RegisterMyInterface(interfaceName string) {
	m.defaultInterfaceAccess.Lock()
	defer m.defaultInterfaceAccess.Unlock()
	m.myInterfaces = append(m.myInterfaces, interfaceName)
}

func (m *platformDefaultInterfaceMonitor) MyInterfaces() []string {
	m.defaultInterfaceAccess.Lock()
	defer m.defaultInterfaceAccess.Unlock()
	return m.myInterfaces
}
