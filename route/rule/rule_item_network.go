package rule

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
	F "github.com/sagernet/sing/common/format"
)

var _ RuleItem = (*NetworkItem)(nil)

type NetworkItem struct {
	networks   []string
	networkMap map[string]bool
}

func NewNetworkItem(networks []string) *NetworkItem {
	networkMap := make(map[string]bool)
	for _, network := range networks {
		networkMap[network] = true
	}
	return &NetworkItem{
		networks:   networks,
		networkMap: networkMap,
	}
}

func (r *NetworkItem) Match(metadata *adapter.InboundContext) bool {
	return r.networkMap[metadata.Network]
}

func (r *NetworkItem) String() string {
	description := "network="

	pLen := len(r.networks)
	if pLen == 1 {
		description += F.ToString(r.networks[0])
	} else {
		description += "[" + strings.Join(F.MapToString(r.networks), " ") + "]"
	}
	return description
}
