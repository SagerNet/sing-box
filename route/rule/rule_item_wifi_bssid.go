package rule

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
	F "github.com/sagernet/sing/common/format"
)

var _ RuleItem = (*WIFIBSSIDItem)(nil)

type WIFIBSSIDItem struct {
	bssidList []string
	bssidMap  map[string]bool
	router    adapter.Router
}

func NewWIFIBSSIDItem(router adapter.Router, bssidList []string) *WIFIBSSIDItem {
	bssidMap := make(map[string]bool)
	for _, bssid := range bssidList {
		bssidMap[bssid] = true
	}
	return &WIFIBSSIDItem{
		bssidList,
		bssidMap,
		router,
	}
}

func (r *WIFIBSSIDItem) Match(metadata *adapter.InboundContext) bool {
	return r.bssidMap[r.router.WIFIState().BSSID]
}

func (r *WIFIBSSIDItem) String() string {
	if len(r.bssidList) == 1 {
		return F.ToString("wifi_bssid=", r.bssidList[0])
	}
	return F.ToString("wifi_bssid=[", strings.Join(r.bssidList, " "), "]")
}
