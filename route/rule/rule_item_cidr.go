package rule

import (
	"net/netip"
	"slices"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/ipset"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json/badoption"

	"go4.org/netipx"
)

var _ RuleItem = (*IPCIDRItem)(nil)

type IPCIDRItem struct {
	ipSet       *ipset.Set
	isSource    bool
	description string
}

func NewIPCIDRItem(isSource bool, prefixables []*badoption.Prefixable) (*IPCIDRItem, error) {
	var builder netipx.IPSetBuilder
	for _, prefixable := range prefixables {
		builder.AddPrefix(prefixable.Build(netip.Prefix{}))
	}
	var description string
	if isSource {
		description = "source_ip_cidr="
	} else {
		description = "ip_cidr="
	}
	prefixString := func(it *badoption.Prefixable) string {
		return it.Build(netip.Prefix{}).String()
	}
	if prefixCount := len(prefixables); prefixCount == 1 {
		description += prefixString(prefixables[0])
	} else if prefixCount > 3 {
		description += "[" + strings.Join(common.Map(prefixables[:3], prefixString), " ") + "...]"
	} else {
		description += "[" + strings.Join(common.Map(prefixables, prefixString), " ") + "]"
	}
	ipSet, err := builder.IPSet()
	if err != nil {
		return nil, err
	}
	return &IPCIDRItem{
		ipSet:       ipset.FromIPSet(ipSet),
		isSource:    isSource,
		description: description,
	}, nil
}

func NewRawIPCIDRItem(isSource bool, ipSet *ipset.Set) *IPCIDRItem {
	var description string
	if isSource {
		description = "source_ip_cidr="
	} else {
		description = "ip_cidr="
	}
	description += "<binary>"
	return &IPCIDRItem{
		ipSet:       ipSet,
		isSource:    isSource,
		description: description,
	}
}

func (r *IPCIDRItem) Match(metadata *adapter.InboundContext) bool {
	if r.isSource || metadata.IPCIDRMatchSource {
		return r.ipSet.Contains(metadata.Source.Addr)
	}
	if metadata.DestinationAddressMatchFromResponse {
		addresses := metadata.DNSResponseAddressesForMatch()
		if len(addresses) == 0 {
			// Legacy rule_set_ip_cidr_accept_empty only applies when the DNS response
			// does not expose any address answers for matching.
			return metadata.IPCIDRAcceptEmpty
		}
		return slices.ContainsFunc(addresses, r.ipSet.Contains)
	}
	if metadata.Destination.IsIP() {
		return r.ipSet.Contains(metadata.Destination.Addr)
	}
	if len(metadata.DestinationAddresses) > 0 {
		return common.Any(metadata.DestinationAddresses, r.ipSet.Contains)
	}
	return metadata.IPCIDRAcceptEmpty
}

func (r *IPCIDRItem) String() string {
	return r.description
}
