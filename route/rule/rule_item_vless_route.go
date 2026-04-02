package rule

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
	F "github.com/sagernet/sing/common/format"
)

var _ RuleItem = (*VLESSRouteItem)(nil)

type VLESSRouteItem struct {
	routes   []uint16
	routeMap map[uint16]bool
}

func NewVLESSRouteItem(routes []uint16) *VLESSRouteItem {
	routeMap := make(map[uint16]bool)
	for _, route := range routes {
		routeMap[route] = true
	}
	return &VLESSRouteItem{
		routes:   routes,
		routeMap: routeMap,
	}
}

func (r *VLESSRouteItem) Match(metadata *adapter.InboundContext) bool {
	return r.routeMap[metadata.VLESSRoute]
}

func (r *VLESSRouteItem) String() string {
	description := "vless_route="
	if len(r.routes) == 1 {
		description += F.ToString(r.routes[0])
	} else {
		description += "[" + strings.Join(F.MapToString(r.routes), " ") + "]"
	}
	return description
}
