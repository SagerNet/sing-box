package rule

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
	F "github.com/sagernet/sing/common/format"
)

var _ RuleItem = (*WIFISSIDItem)(nil)

type WIFISSIDItem struct {
	ssidList       []string
	ssidMap        map[string]bool
	networkManager adapter.NetworkManager
}

func NewWIFISSIDItem(networkManager adapter.NetworkManager, ssidList []string) *WIFISSIDItem {
	ssidMap := make(map[string]bool)
	for _, ssid := range ssidList {
		ssidMap[ssid] = true
	}
	return &WIFISSIDItem{
		ssidList,
		ssidMap,
		networkManager,
	}
}

func (r *WIFISSIDItem) Match(metadata *adapter.InboundContext) bool {
	return r.ssidMap[r.networkManager.WIFIState().SSID]
}

func (r *WIFISSIDItem) String() string {
	if len(r.ssidList) == 1 {
		return F.ToString("wifi_ssid=", r.ssidList[0])
	}
	return F.ToString("wifi_ssid=[", strings.Join(r.ssidList, " "), "]")
}
