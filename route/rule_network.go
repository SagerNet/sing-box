package route

import (
	"github.com/sagernet/sing-box/adapter"
)

var _ RuleItem = (*NetworkItem)(nil)

type NetworkItem struct {
	network string
}

func NewNetworkItem(network string) *NetworkItem {
	return &NetworkItem{network}
}

func (r *NetworkItem) Match(metadata *adapter.InboundContext) bool {
	return r.network == metadata.Network
}

func (r *NetworkItem) String() string {
	return "network=" + r.network
}
