package route

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
)

var _ adapter.Rule = (*LogicalRule)(nil)

type LogicalRule struct {
	mode     string
	rules    []*DefaultRule
	outbound string
}

func NewLogicalRule(router adapter.Router, logger log.Logger, options option.LogicalRule) (*LogicalRule, error) {
	r := &LogicalRule{
		rules:    make([]*DefaultRule, len(options.Rules)),
		outbound: options.Outbound,
	}
	switch options.Mode {
	case C.LogicalTypeAnd:
		r.mode = C.LogicalTypeAnd
	case C.LogicalTypeOr:
		r.mode = C.LogicalTypeOr
	default:
		return nil, E.New("unknown logical mode: ", options.Mode)
	}
	for i, subRule := range options.Rules {
		rule, err := NewDefaultRule(router, logger, subRule)
		if err != nil {
			return nil, E.Cause(err, "sub rule[", i, "]")
		}
		r.rules[i] = rule
	}
	return r, nil
}

func (r *LogicalRule) Match(metadata *adapter.InboundContext) bool {
	if r.mode == C.LogicalTypeAnd {
		return common.All(r.rules, func(it *DefaultRule) bool {
			return it.Match(metadata)
		})
	} else {
		return common.Any(r.rules, func(it *DefaultRule) bool {
			return it.Match(metadata)
		})
	}
}

func (r *LogicalRule) Outbound() string {
	return r.outbound
}

func (r *LogicalRule) String() string {
	var op string
	switch r.mode {
	case C.LogicalTypeAnd:
		op = "&&"
	case C.LogicalTypeOr:
		op = "||"
	}
	return "logical(" + strings.Join(common.Map(r.rules, F.ToString0[*DefaultRule]), " "+op+" ") + ")"
}
