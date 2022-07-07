package route

import (
	"strings"

	F "github.com/sagernet/sing/common/format"

	"github.com/sagernet/sing-box/adapter"
)

var _ RuleItem = (*OutboundItem)(nil)

type OutboundItem struct {
	outbounds   []string
	outboundMap map[string]bool
}

func NewOutboundRule(outbounds []string) *OutboundItem {
	rule := &OutboundItem{outbounds, make(map[string]bool)}
	for _, outbound := range outbounds {
		rule.outboundMap[outbound] = true
	}
	return rule
}

func (r *OutboundItem) Match(metadata *adapter.InboundContext) bool {
	return r.outboundMap[metadata.Outbound]
}

func (r *OutboundItem) String() string {
	if len(r.outbounds) == 1 {
		return F.ToString("outbound=", r.outbounds[0])
	} else {
		return F.ToString("outbound=[", strings.Join(r.outbounds, " "), "]")
	}
}
