package libbox

import (
	"net"
	"net/netip"
	"sync"

	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/x/list"
)

var (
	_ tun.DefaultInterfaceMonitor = (*platformDefaultInterfaceMonitor)(nil)
	_ InterfaceUpdateListener     = (*platformDefaultInterfaceMonitor)(nil)
)

type platformDefaultInterfaceMonitor struct {
	*platformInterfaceWrapper
	networkAddresses      []networkAddress
	defaultInterfaceName  string
	defaultInterfaceIndex int
	element               *list.Element[tun.NetworkUpdateCallback]
	access                sync.Mutex
	callbacks             list.List[tun.DefaultInterfaceUpdateCallback]
	logger                logger.Logger
}

type networkAddress struct {
	interfaceName  string
	interfaceIndex int
	addresses      []netip.Prefix
}

func (m *platformDefaultInterfaceMonitor) Start() error {
	return m.iif.StartDefaultInterfaceMonitor(m)
}

func (m *platformDefaultInterfaceMonitor) Close() error {
	return m.iif.CloseDefaultInterfaceMonitor(m)
}

func (m *platformDefaultInterfaceMonitor) DefaultInterfaceName(destination netip.Addr) string {
	for _, address := range m.networkAddresses {
		for _, prefix := range address.addresses {
			if prefix.Contains(destination) {
				return address.interfaceName
			}
		}
	}
	return m.defaultInterfaceName
}

func (m *platformDefaultInterfaceMonitor) DefaultInterfaceIndex(destination netip.Addr) int {
	for _, address := range m.networkAddresses {
		for _, prefix := range address.addresses {
			if prefix.Contains(destination) {
				return address.interfaceIndex
			}
		}
	}
	return m.defaultInterfaceIndex
}

func (m *platformDefaultInterfaceMonitor) DefaultInterface(destination netip.Addr) (string, int) {
	for _, address := range m.networkAddresses {
		for _, prefix := range address.addresses {
			if prefix.Contains(destination) {
				return address.interfaceName, address.interfaceIndex
			}
		}
	}
	return m.defaultInterfaceName, m.defaultInterfaceIndex
}

func (m *platformDefaultInterfaceMonitor) OverrideAndroidVPN() bool {
	return false
}

func (m *platformDefaultInterfaceMonitor) AndroidVPNEnabled() bool {
	return false
}

func (m *platformDefaultInterfaceMonitor) RegisterCallback(callback tun.DefaultInterfaceUpdateCallback) *list.Element[tun.DefaultInterfaceUpdateCallback] {
	m.access.Lock()
	defer m.access.Unlock()
	return m.callbacks.PushBack(callback)
}

func (m *platformDefaultInterfaceMonitor) UnregisterCallback(element *list.Element[tun.DefaultInterfaceUpdateCallback]) {
	m.access.Lock()
	defer m.access.Unlock()
	m.callbacks.Remove(element)
}

func (m *platformDefaultInterfaceMonitor) UpdateDefaultInterface(interfaceName string, interfaceIndex32 int32) {
	if interfaceName == "" || interfaceIndex32 == -1 {
		m.defaultInterfaceName = ""
		m.defaultInterfaceIndex = -1
		m.access.Lock()
		callbacks := m.callbacks.Array()
		m.access.Unlock()
		for _, callback := range callbacks {
			callback(tun.EventNoRoute)
		}
		return
	}
	var err error
	if m.iif.UsePlatformInterfaceGetter() {
		err = m.updateInterfacesPlatform()
	} else {
		err = m.updateInterfaces()
	}
	if err == nil {
		err = m.networkManager.UpdateInterfaces()
	}
	if err != nil {
		m.logger.Error(E.Cause(err, "update interfaces"))
	}
	interfaceIndex := int(interfaceIndex32)
	if m.defaultInterfaceName == interfaceName && m.defaultInterfaceIndex == interfaceIndex {
		return
	}
	m.defaultInterfaceName = interfaceName
	m.defaultInterfaceIndex = interfaceIndex
	m.access.Lock()
	callbacks := m.callbacks.Array()
	m.access.Unlock()
	for _, callback := range callbacks {
		callback(tun.EventInterfaceUpdate)
	}
}

func (m *platformDefaultInterfaceMonitor) updateInterfaces() error {
	interfaces, err := net.Interfaces()
	if err != nil {
		return err
	}
	var addresses []networkAddress
	for _, iif := range interfaces {
		var netAddresses []net.Addr
		netAddresses, err = iif.Addrs()
		if err != nil {
			return err
		}
		var address networkAddress
		address.interfaceName = iif.Name
		address.interfaceIndex = iif.Index
		address.addresses = common.Map(common.FilterIsInstance(netAddresses, func(it net.Addr) (*net.IPNet, bool) {
			value, loaded := it.(*net.IPNet)
			return value, loaded
		}), func(it *net.IPNet) netip.Prefix {
			bits, _ := it.Mask.Size()
			return netip.PrefixFrom(M.AddrFromIP(it.IP), bits)
		})
		addresses = append(addresses, address)
	}
	m.networkAddresses = addresses
	return nil
}

func (m *platformDefaultInterfaceMonitor) updateInterfacesPlatform() error {
	interfaces, err := m.Interfaces()
	if err != nil {
		return err
	}
	var addresses []networkAddress
	for _, iif := range interfaces {
		var address networkAddress
		address.interfaceName = iif.Name
		address.interfaceIndex = iif.Index
		// address.addresses = common.Map(iif.Addresses, netip.MustParsePrefix)
		addresses = append(addresses, address)
	}
	m.networkAddresses = addresses
	return nil
}
