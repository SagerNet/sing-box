package route

import (
	"context"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/json"
	"github.com/sagernet/sing-box/common/srs"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

var _ adapter.RuleSet = (*LocalRuleSet)(nil)

type LocalRuleSet struct {
	rules []adapter.HeadlessRule
}

func NewLocalRuleSet(router adapter.Router, options option.RuleSet) (*LocalRuleSet, error) {
	setFile, err := os.Open(options.LocalOptions.Path)
	if err != nil {
		return nil, err
	}
	var plainRuleSet option.PlainRuleSet
	switch options.Format {
	case C.RuleSetFormatSource, "":
		var compat option.PlainRuleSetCompat
		decoder := json.NewDecoder(json.NewCommentFilter(setFile))
		decoder.DisallowUnknownFields()
		err = decoder.Decode(&compat)
		if err != nil {
			return nil, err
		}
		plainRuleSet = compat.Upgrade()
	case C.RuleSetFormatBinary:
		plainRuleSet, err = srs.Read(setFile, false)
		if err != nil {
			return nil, err
		}
	default:
		return nil, E.New("unknown rule set format: ", options.Format)
	}
	rules := make([]adapter.HeadlessRule, len(plainRuleSet.Rules))
	for i, ruleOptions := range plainRuleSet.Rules {
		rules[i], err = NewHeadlessRule(router, ruleOptions)
		if err != nil {
			return nil, E.Cause(err, "parse rule_set.rules.[", i, "]")
		}
	}
	return &LocalRuleSet{rules}, nil
}

func (s *LocalRuleSet) Match(metadata *adapter.InboundContext) bool {
	for _, rule := range s.rules {
		if rule.Match(metadata) {
			return true
		}
	}
	return false
}

func (s *LocalRuleSet) StartContext(ctx context.Context, startContext adapter.RuleSetStartContext) error {
	return nil
}

func (s *LocalRuleSet) Close() error {
	return nil
}
