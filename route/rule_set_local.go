package route

import (
	"context"
	"os"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/srs"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/atomic"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/x/list"

	"go4.org/netipx"
)

var _ adapter.RuleSet = (*LocalRuleSet)(nil)

type LocalRuleSet struct {
	tag      string
	rules    []adapter.HeadlessRule
	metadata adapter.RuleSetMetadata
	refs     atomic.Int32
}

func NewLocalRuleSet(router adapter.Router, options option.RuleSet) (*LocalRuleSet, error) {
	var plainRuleSet option.PlainRuleSet
	switch options.Format {
	case C.RuleSetFormatSource, "":
		content, err := os.ReadFile(options.LocalOptions.Path)
		if err != nil {
			return nil, err
		}
		compat, err := json.UnmarshalExtended[option.PlainRuleSetCompat](content)
		if err != nil {
			return nil, err
		}
		plainRuleSet = compat.Upgrade()
	case C.RuleSetFormatBinary:
		setFile, err := os.Open(options.LocalOptions.Path)
		if err != nil {
			return nil, err
		}
		plainRuleSet, err = srs.Read(setFile, false)
		if err != nil {
			return nil, err
		}
	default:
		return nil, E.New("unknown rule set format: ", options.Format)
	}
	rules := make([]adapter.HeadlessRule, len(plainRuleSet.Rules))
	var err error
	for i, ruleOptions := range plainRuleSet.Rules {
		rules[i], err = NewHeadlessRule(router, ruleOptions)
		if err != nil {
			return nil, E.Cause(err, "parse rule_set.rules.[", i, "]")
		}
	}
	var metadata adapter.RuleSetMetadata
	metadata.ContainsProcessRule = hasHeadlessRule(plainRuleSet.Rules, isProcessHeadlessRule)
	metadata.ContainsWIFIRule = hasHeadlessRule(plainRuleSet.Rules, isWIFIHeadlessRule)
	metadata.ContainsIPCIDRRule = hasHeadlessRule(plainRuleSet.Rules, isIPCIDRHeadlessRule)
	return &LocalRuleSet{tag: options.Tag, rules: rules, metadata: metadata}, nil
}

func (s *LocalRuleSet) Name() string {
	return s.tag
}

func (s *LocalRuleSet) String() string {
	return strings.Join(F.MapToString(s.rules), " ")
}

func (s *LocalRuleSet) StartContext(ctx context.Context, startContext adapter.RuleSetStartContext) error {
	return nil
}

func (s *LocalRuleSet) PostStart() error {
	return nil
}

func (s *LocalRuleSet) Metadata() adapter.RuleSetMetadata {
	return s.metadata
}

func (s *LocalRuleSet) ExtractIPSet() []*netipx.IPSet {
	return common.FlatMap(s.rules, extractIPSetFromRule)
}

func (s *LocalRuleSet) IncRef() {
	s.refs.Add(1)
}

func (s *LocalRuleSet) DecRef() {
	if s.refs.Add(-1) < 0 {
		panic("rule-set: negative refs")
	}
}

func (s *LocalRuleSet) Cleanup() {
	if s.refs.Load() == 0 {
		s.rules = nil
	}
}

func (s *LocalRuleSet) RegisterCallback(callback adapter.RuleSetUpdateCallback) *list.Element[adapter.RuleSetUpdateCallback] {
	return nil
}

func (s *LocalRuleSet) UnregisterCallback(element *list.Element[adapter.RuleSetUpdateCallback]) {
}

func (s *LocalRuleSet) Close() error {
	s.rules = nil
	return nil
}

func (s *LocalRuleSet) Match(metadata *adapter.InboundContext) bool {
	for _, rule := range s.rules {
		if rule.Match(metadata) {
			return true
		}
	}
	return false
}
