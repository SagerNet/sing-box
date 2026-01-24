package rule

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
)

var _ RuleItem = (*QueryTypeItem)(nil)

type QueryTypeItem struct {
	typeList []uint16
	typeMap  map[uint16]bool
}

func NewQueryTypeItem(typeList []option.DNSQueryType) *QueryTypeItem {
	rule := &QueryTypeItem{
		typeList: common.Map(typeList, func(it option.DNSQueryType) uint16 {
			return uint16(it)
		}),
		typeMap: make(map[uint16]bool),
	}
	for _, userId := range rule.typeList {
		rule.typeMap[userId] = true
	}
	return rule
}

func (r *QueryTypeItem) Match(metadata *adapter.InboundContext) bool {
	if metadata.QueryType == 0 {
		return false
	}
	return r.typeMap[metadata.QueryType]
}

func (r *QueryTypeItem) String() string {
	var description string
	pLen := len(r.typeList)
	if pLen == 1 {
		description = "query_type=" + option.DNSQueryTypeToString(r.typeList[0])
	} else {
		description = "query_type=[" + strings.Join(common.Map(r.typeList, option.DNSQueryTypeToString), " ") + "]"
	}
	return description
}
