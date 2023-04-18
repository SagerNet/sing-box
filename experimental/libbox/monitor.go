package libbox

import (
	"context"
	"net"
	"net/netip"
	"sync"

	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/x/list"
)

var (
	_ tun.DefaultInterfaceMonitor = (*platformDefaultInterfaceMonitor)(nil)
	_ InterfaceUpdateListener     = (*platformDefaultInterfaceMonitor)(nil)
)

type platformDefaultInterfaceMonitor struct {
	*platformInterfaceWrapper
	errorHandler          E.Handler
	networkAddresses      []networkAddress
	defaultInterfaceName  string
	defaultInterfaceIndex int
	element               *list.Element[tun.NetworkUpdateCallback]
	access                sync.Mutex
	callbacks             list.List[tun.DefaultInterfaceUpdateCallback]
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
	var err error
	if m.iif.UsePlatformInterfaceGetter() {
		err = m.updateInterfacesPlatform()
	} else {
		err = m.updateInterfaces()
	}
	if err == nil {
		err = m.router.UpdateInterfaces()
	}
	if err != nil {
		m.errorHandler.NewError(context.Background(), E.Cause(err, "update interfaces"))
	}
	interfaceIndex := int(interfaceIndex32)
	if interfaceName == "" {
		for _, netIf := range m.networkAddresses {
			if netIf.interfaceIndex == interfaceIndex {
				interfaceName = netIf.interfaceName
				break
			}
		}
	} else if interfaceIndex == -1 {
		for _, netIf := range m.networkAddresses {
			if netIf.interfaceName == interfaceName {
				interfaceIndex = netIf.interfaceIndex
				break
			}
		}
	}
	if interfaceName == "" {
		m.errorHandler.NewError(context.Background(), E.New("invalid interface name for ", interfaceIndex))
		return
	} else if interfaceIndex == -1 {
		m.errorHandler.NewError(context.Background(), E.New("invalid interface index for ", interfaceName))
		return
	}
	if m.defaultInterfaceName == interfaceName && m.defaultInterfaceIndex == interfaceIndex {
		return
	}
	m.defaultInterfaceName = interfaceName
	m.defaultInterfaceIndex = interfaceIndex
	m.access.Lock()
	callbacks := m.callbacks.Array()
	m.access.Unlock()
	for _, callback := range callbacks {
		err = callback(tun.EventInterfaceUpdate)
		if err != nil {
			m.errorHandler.NewError(context.Background(), err)
		}
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
