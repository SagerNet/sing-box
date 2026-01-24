package rule

import (
	"github.com/sagernet/sing-box/adapter"
)

var _ RuleItem = (*NetworkIsExpensiveItem)(nil)

type NetworkIsExpensiveItem struct {
	networkManager adapter.NetworkManager
}

func NewNetworkIsExpensiveItem(networkManager adapter.NetworkManager) *NetworkIsExpensiveItem {
	return &NetworkIsExpensiveItem{
		networkManager: networkManager,
	}
}

func (r *NetworkIsExpensiveItem) Match(metadata *adapter.InboundContext) bool {
	networkInterface := r.networkManager.DefaultNetworkInterface()
	if networkInterface == nil {
		return false
	}
	return networkInterface.Expensive
}

func (r *NetworkIsExpensiveItem) String() string {
	return "network_is_expensive=true"
}
