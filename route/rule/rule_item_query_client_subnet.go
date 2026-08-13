package rule

import (
	"net/netip"
	"slices"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json/badoption"
)

var _ RuleItem = (*QueryClientSubnetItem)(nil)

type QueryClientSubnetItem struct {
	prefixes []netip.Prefix
}

func NewQueryClientSubnetItem(prefixables badoption.Listable[*badoption.Prefixable]) *QueryClientSubnetItem {
	return &QueryClientSubnetItem{
		prefixes: common.Map(prefixables, func(it *badoption.Prefixable) netip.Prefix {
			return it.Build(netip.Prefix{})
		}),
	}
}

func (r *QueryClientSubnetItem) Match(metadata *adapter.InboundContext) bool {
	clientSubnet := metadata.QueryClientSubnet
	if !clientSubnet.IsValid() {
		return false
	}
	return slices.ContainsFunc(r.prefixes, func(prefix netip.Prefix) bool {
		return clientSubnet.Bits() >= prefix.Bits() && prefix.Contains(clientSubnet.Addr())
	})
}

func (r *QueryClientSubnetItem) String() string {
	if len(r.prefixes) == 1 {
		return "query_client_subnet=" + r.prefixes[0].String()
	}
	return "query_client_subnet=[" + strings.Join(common.Map(r.prefixes, netip.Prefix.String), " ") + "]"
}
