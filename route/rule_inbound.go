package route

import (
	"strings"

	F "github.com/sagernet/sing/common/format"

	"github.com/sagernet/sing-box/adapter"
)

var _ RuleItem = (*InboundItem)(nil)

type InboundItem struct {
	inbounds   []string
	inboundMap map[string]bool
}

func NewInboundRule(inbounds []string) *InboundItem {
	rule := &InboundItem{inbounds, make(map[string]bool)}
	for _, inbound := range inbounds {
		rule.inboundMap[inbound] = true
	}
	return rule
}

func (r *InboundItem) Match(metadata *adapter.InboundContext) bool {
	return r.inboundMap[metadata.Inbound]
}

func (r *InboundItem) String() string {
	if len(r.inbounds) == 1 {
		return F.ToString("inbound=", r.inbounds[0])
	} else {
		return F.ToString("inbound=[", strings.Join(r.inbounds, " "), "]")
	}
}
