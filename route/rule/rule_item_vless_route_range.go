package rule

import (
	"strconv"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
)

var ErrBadVLESSRouteRange = E.New("bad VLESS route range")

var _ RuleItem = (*VLESSRouteRangeItem)(nil)

type VLESSRouteRangeItem struct {
	rangeStrings []string
	rangeList    []rangeItem
}

func NewVLESSRouteRangeItem(rangeStrings []string) (*VLESSRouteRangeItem, error) {
	rangeList := make([]rangeItem, 0, len(rangeStrings))
	for _, rangeStr := range rangeStrings {
		if !strings.Contains(rangeStr, ":") {
			return nil, E.Extend(ErrBadVLESSRouteRange, rangeStr)
		}
		subIndex := strings.Index(rangeStr, ":")
		var start, end uint64
		var err error
		if subIndex > 0 {
			start, err = strconv.ParseUint(rangeStr[:subIndex], 10, 16)
			if err != nil {
				return nil, E.Cause(err, E.Extend(ErrBadVLESSRouteRange, rangeStr))
			}
		}
		if subIndex == len(rangeStr)-1 {
			end = 0xFFFF
		} else {
			end, err = strconv.ParseUint(rangeStr[subIndex+1:], 10, 16)
			if err != nil {
				return nil, E.Cause(err, E.Extend(ErrBadVLESSRouteRange, rangeStr))
			}
		}
		rangeList = append(rangeList, rangeItem{uint16(start), uint16(end)})
	}
	return &VLESSRouteRangeItem{
		rangeStrings: rangeStrings,
		rangeList:    rangeList,
	}, nil
}

func (r *VLESSRouteRangeItem) Match(metadata *adapter.InboundContext) bool {
	route := metadata.VLESSRoute
	for _, routeRange := range r.rangeList {
		if route >= routeRange.start && route <= routeRange.end {
			return true
		}
	}
	return false
}

func (r *VLESSRouteRangeItem) String() string {
	description := "vless_route_range="
	if len(r.rangeStrings) == 1 {
		description += r.rangeStrings[0]
	} else {
		description += "[" + strings.Join(r.rangeStrings, " ") + "]"
	}
	return description
}
