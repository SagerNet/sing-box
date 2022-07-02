package route

import (
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	F "github.com/sagernet/sing/common/format"
)

var _ adapter.Rule = (*DefaultRule)(nil)

type DefaultRule struct {
	index    int
	outbound string
	items    []RuleItem
}

type RuleItem interface {
	Match(metadata adapter.InboundContext) bool
	String() string
}

func NewDefaultRule(index int, options option.DefaultRule) *DefaultRule {
	rule := &DefaultRule{
		index:    index,
		outbound: options.Outbound,
	}
	if len(options.Inbound) > 0 {
		rule.items = append(rule.items, NewInboundRule(options.Inbound))
	}
	return rule
}

func (r *DefaultRule) Match(metadata adapter.InboundContext) bool {
	for _, item := range r.items {
		if item.Match(metadata) {
			return true
		}
	}
	return false
}

func (r *DefaultRule) Outbound() string {
	return r.outbound
}

func (r *DefaultRule) String() string {
	var description string
	description = F.ToString("[", r.index, "]")
	for _, item := range r.items {
		description += " "
		description += item.String()
	}
	description += " => " + r.outbound
	return description
}
