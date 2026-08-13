package rule

import (
	"github.com/sagernet/sing-box/adapter"
)

var _ RuleItem = (*QueryDNSSECItem)(nil)

type QueryDNSSECItem struct{}

func NewQueryDNSSECItem() *QueryDNSSECItem {
	return &QueryDNSSECItem{}
}

func (r *QueryDNSSECItem) Match(metadata *adapter.InboundContext) bool {
	return metadata.QueryDNSSEC
}

func (r *QueryDNSSECItem) String() string {
	return "query_dnssec=true"
}
