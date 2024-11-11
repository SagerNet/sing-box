package rule

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common"
	F "github.com/sagernet/sing/common/format"
)

var _ RuleItem = (*NetworkTypeItem)(nil)

type NetworkTypeItem struct {
	networkManager adapter.NetworkManager
	networkType    []string
}

func NewNetworkTypeItem(networkManager adapter.NetworkManager, networkType []string) *NetworkTypeItem {
	return &NetworkTypeItem{
		networkManager: networkManager,
		networkType:    networkType,
	}
}

func (r *NetworkTypeItem) Match(metadata *adapter.InboundContext) bool {
	networkInterface := r.networkManager.DefaultNetworkInterface()
	if networkInterface == nil {
		return false
	}
	return common.Contains(r.networkType, networkInterface.Type)
}

func (r *NetworkTypeItem) String() string {
	if len(r.networkType) == 1 {
		return F.ToString("network_type=", r.networkType[0])
	} else {
		return F.ToString("network_type=", "["+strings.Join(F.MapToString(r.networkType), " ")+"]")
	}
}
