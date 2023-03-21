package route

import (
	"strconv"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
)

var ErrBadPortRange = E.New("bad port range")

var _ RuleItem = (*PortRangeItem)(nil)

type PortRangeItem struct {
	isSource      bool
	portRanges    []string
	portRangeList []rangeItem
}

type rangeItem struct {
	start uint16
	end   uint16
}

func NewPortRangeItem(isSource bool, rangeList []string) (*PortRangeItem, error) {
	portRangeList := make([]rangeItem, 0, len(rangeList))
	for _, portRange := range rangeList {
		if !strings.Contains(portRange, ":") {
			return nil, E.Extend(ErrBadPortRange, portRange)
		}
		subIndex := strings.Index(portRange, ":")
		var start, end uint64
		var err error
		if subIndex > 0 {
			start, err = strconv.ParseUint(portRange[:subIndex], 10, 16)
			if err != nil {
				return nil, E.Cause(err, E.Extend(ErrBadPortRange, portRange))
			}
		}
		if subIndex == len(portRange)-1 {
			end = 0xFF
		} else {
			end, err = strconv.ParseUint(portRange[subIndex+1:], 10, 16)
			if err != nil {
				return nil, E.Cause(err, E.Extend(ErrBadPortRange, portRange))
			}
		}
		portRangeList = append(portRangeList, rangeItem{uint16(start), uint16(end)})
	}
	return &PortRangeItem{
		isSource:      isSource,
		portRanges:    rangeList,
		portRangeList: portRangeList,
	}, nil
}

func (r *PortRangeItem) Match(metadata *adapter.InboundContext) bool {
	var port uint16
	if r.isSource {
		port = metadata.Source.Port
	} else {
		port = metadata.Destination.Port
	}
	for _, portRange := range r.portRangeList {
		if port >= portRange.start && port <= portRange.end {
			return true
		}
	}
	return false
}

func (r *PortRangeItem) String() string {
	var description string
	if r.isSource {
		description = "source_port_range="
	} else {
		description = "port_range="
	}
	pLen := len(r.portRanges)
	if pLen == 1 {
		description += r.portRanges[0]
	} else {
		description += "[" + strings.Join(r.portRanges, " ") + "]"
	}
	return description
}
