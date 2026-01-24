package rule

import (
	"github.com/sagernet/sing-box/adapter"
)

var _ RuleItem = (*NetworkIsConstrainedItem)(nil)

type NetworkIsConstrainedItem struct {
	networkManager adapter.NetworkManager
}

func NewNetworkIsConstrainedItem(networkManager adapter.NetworkManager) *NetworkIsConstrainedItem {
	return &NetworkIsConstrainedItem{
		networkManager: networkManager,
	}
}

func (r *NetworkIsConstrainedItem) Match(metadata *adapter.InboundContext) bool {
	networkInterface := r.networkManager.DefaultNetworkInterface()
	if networkInterface == nil {
		return false
	}
	return networkInterface.Constrained
}

func (r *NetworkIsConstrainedItem) String() string {
	return "network_is_expensive=true"
}
