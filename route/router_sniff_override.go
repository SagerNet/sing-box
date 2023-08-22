package route

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
)

func (r *Router) matchSniffOverride(ctx context.Context, metadata *adapter.InboundContext) bool {
	rules := make([]adapter.SniffOverrideRule, 0, len(metadata.InboundOptions.SniffOverrideRules))
	for i, sniffOverrideRuleOptions := range metadata.InboundOptions.SniffOverrideRules {
		sniffOverrideRule, err := NewSniffOverrideRule(r, r.logger, sniffOverrideRuleOptions)
		if err != nil {
			E.Cause(err, "parse sniff_override rule[", i, "]")
			return false
		}
		rules = append(rules, sniffOverrideRule)
	}
	if len(rules) == 0 {
		r.overrideLogger.DebugContext(ctx, "match all")
		return true
	}
	for i, rule := range rules {
		if rule.Match(metadata) {
			r.overrideLogger.DebugContext(ctx, "match[", i, "] ", rule.String())
			return true
		}
	}
	return false
}
