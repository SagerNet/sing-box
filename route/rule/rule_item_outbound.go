package rule

import (
	"context"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/experimental/deprecated"
	F "github.com/sagernet/sing/common/format"
)

var _ RuleItem = (*OutboundItem)(nil)

type OutboundItem struct {
	outbounds   []string
	outboundMap map[string]bool
	matchAny    bool
}

func NewOutboundRule(ctx context.Context, outbounds []string) *OutboundItem {
	deprecated.Report(ctx, deprecated.OptionOutboundDNSRuleItem)
	rule := &OutboundItem{outbounds: outbounds, outboundMap: make(map[string]bool)}
	for _, outbound := range outbounds {
		if outbound == "any" {
			rule.matchAny = true
		} else {
			rule.outboundMap[outbound] = true
		}
	}
	return rule
}

func (r *OutboundItem) Match(metadata *adapter.InboundContext) bool {
	if r.matchAny {
		return metadata.Outbound != ""
	}
	return r.outboundMap[metadata.Outbound]
}

func (r *OutboundItem) String() string {
	if len(r.outbounds) == 1 {
		return F.ToString("outbound=", r.outbounds[0])
	} else {
		return F.ToString("outbound=[", strings.Join(r.outbounds, " "), "]")
	}
}
