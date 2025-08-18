package rule

import (
	"net/netip"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json/badoption"
)

var _ RuleItem = (*DefaultInterfaceAddressItem)(nil)

type DefaultInterfaceAddressItem struct {
	interfaceMonitor   tun.DefaultInterfaceMonitor
	interfaceAddresses []netip.Prefix
}

func NewDefaultInterfaceAddressItem(networkManager adapter.NetworkManager, interfaceAddresses badoption.Listable[*badoption.Prefixable]) *DefaultInterfaceAddressItem {
	item := &DefaultInterfaceAddressItem{
		interfaceMonitor:   networkManager.InterfaceMonitor(),
		interfaceAddresses: make([]netip.Prefix, 0, len(interfaceAddresses)),
	}
	for _, prefixable := range interfaceAddresses {
		item.interfaceAddresses = append(item.interfaceAddresses, prefixable.Build(netip.Prefix{}))
	}
	return item
}

func (r *DefaultInterfaceAddressItem) Match(metadata *adapter.InboundContext) bool {
	defaultInterface := r.interfaceMonitor.DefaultInterface()
	if defaultInterface == nil {
		return false
	}
	for _, address := range r.interfaceAddresses {
		if common.All(defaultInterface.Addresses, func(it netip.Prefix) bool {
			return !address.Overlaps(it)
		}) {
			return false
		}
	}
	return true
}

func (r *DefaultInterfaceAddressItem) String() string {
	addressLen := len(r.interfaceAddresses)
	switch {
	case addressLen == 1:
		return "default_interface_address=" + r.interfaceAddresses[0].String()
	case addressLen > 3:
		return "default_interface_address=[" + strings.Join(common.Map(r.interfaceAddresses[:3], netip.Prefix.String), " ") + "...]"
	default:
		return "default_interface_address=[" + strings.Join(common.Map(r.interfaceAddresses, netip.Prefix.String), " ") + "]"
	}
}
