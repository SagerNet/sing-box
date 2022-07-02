package route

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
	F "github.com/sagernet/sing/common/format"
)

var _ RuleItem = (*InboundRule)(nil)

type InboundRule struct {
	inbounds   []string
	inboundMap map[string]bool
}

func NewInboundRule(inbounds []string) RuleItem {
	rule := &InboundRule{inbounds, make(map[string]bool)}
	for _, inbound := range inbounds {
		rule.inboundMap[inbound] = true
	}
	return rule
}

func (r *InboundRule) Match(metadata adapter.InboundContext) bool {
	return r.inboundMap[metadata.Inbound]
}

func (r *InboundRule) String() string {
	if len(r.inbounds) == 1 {
		return F.ToString("inbound=", r.inbounds[0])
	} else {
		return F.ToString("inbound=[", strings.Join(r.inbounds, " "), "]")
	}
}
