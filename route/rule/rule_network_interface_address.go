package rule

import (
	"net/netip"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
)

var _ RuleItem = (*NetworkInterfaceAddressItem)(nil)

type NetworkInterfaceAddressItem struct {
	networkManager     adapter.NetworkManager
	interfaceAddresses map[C.InterfaceType][]netip.Prefix
	description        string
}

func NewNetworkInterfaceAddressItem(networkManager adapter.NetworkManager, interfaceAddresses *badjson.TypedMap[option.InterfaceType, badoption.Listable[*badoption.Prefixable]]) *NetworkInterfaceAddressItem {
	item := &NetworkInterfaceAddressItem{
		networkManager:     networkManager,
		interfaceAddresses: make(map[C.InterfaceType][]netip.Prefix, interfaceAddresses.Size()),
	}
	var entryDescriptions []string
	for _, entry := range interfaceAddresses.Entries() {
		prefixes := make([]netip.Prefix, 0, len(entry.Value))
		for _, prefixable := range entry.Value {
			prefixes = append(prefixes, prefixable.Build(netip.Prefix{}))
		}
		item.interfaceAddresses[entry.Key.Build()] = prefixes
		entryDescriptions = append(entryDescriptions, entry.Key.Build().String()+"="+strings.Join(common.Map(prefixes, netip.Prefix.String), ","))
	}
	item.description = "network_interface_address=[" + strings.Join(entryDescriptions, " ") + "]"
	return item
}

func (r *NetworkInterfaceAddressItem) Match(metadata *adapter.InboundContext) bool {
	interfaces := r.networkManager.NetworkInterfaces()
match:
	for ifType, addresses := range r.interfaceAddresses {
		for _, networkInterface := range interfaces {
			if networkInterface.Type != ifType {
				continue
			}
			if common.Any(networkInterface.Addresses, func(it netip.Prefix) bool {
				return common.Any(addresses, func(prefix netip.Prefix) bool {
					return prefix.Overlaps(it)
				})
			}) {
				continue match
			}
		}
		return false
	}
	return true
}

func (r *NetworkInterfaceAddressItem) String() string {
	return r.description
}
