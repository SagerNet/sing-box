package rule

import (
	"net/netip"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/control"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
)

var _ RuleItem = (*InterfaceAddressItem)(nil)

type InterfaceAddressItem struct {
	networkManager     adapter.NetworkManager
	interfaceAddresses map[string][]netip.Prefix
	description        string
}

func NewInterfaceAddressItem(networkManager adapter.NetworkManager, interfaceAddresses *badjson.TypedMap[string, badoption.Listable[*badoption.Prefixable]]) *InterfaceAddressItem {
	item := &InterfaceAddressItem{
		networkManager:     networkManager,
		interfaceAddresses: make(map[string][]netip.Prefix, interfaceAddresses.Size()),
	}
	var entryDescriptions []string
	for _, entry := range interfaceAddresses.Entries() {
		prefixes := make([]netip.Prefix, 0, len(entry.Value))
		for _, prefixable := range entry.Value {
			prefixes = append(prefixes, prefixable.Build(netip.Prefix{}))
		}
		item.interfaceAddresses[entry.Key] = prefixes
		entryDescriptions = append(entryDescriptions, entry.Key+"="+strings.Join(common.Map(prefixes, netip.Prefix.String), ","))
	}
	item.description = "interface_address=[" + strings.Join(entryDescriptions, " ") + "]"
	return item
}

func (r *InterfaceAddressItem) Match(metadata *adapter.InboundContext) bool {
	interfaces := r.networkManager.InterfaceFinder().Interfaces()
	for ifName, addresses := range r.interfaceAddresses {
		iface := common.Find(interfaces, func(it control.Interface) bool {
			return it.Name == ifName
		})
		if iface.Name == "" {
			return false
		}
		if common.All(addresses, func(address netip.Prefix) bool {
			return common.All(iface.Addresses, func(it netip.Prefix) bool {
				return !address.Overlaps(it)
			})
		}) {
			return false
		}
	}
	return true
}

func (r *InterfaceAddressItem) String() string {
	return r.description
}
