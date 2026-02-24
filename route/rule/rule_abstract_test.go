package rule

import (
	"context"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common/x/list"

	"github.com/stretchr/testify/require"
	"go4.org/netipx"
)

type fakeRuleSet struct {
	matched bool
}

func (f *fakeRuleSet) Name() string {
	return "fake-rule-set"
}

func (f *fakeRuleSet) StartContext(context.Context, *adapter.HTTPStartContext) error {
	return nil
}

func (f *fakeRuleSet) PostStart() error {
	return nil
}

func (f *fakeRuleSet) Metadata() adapter.RuleSetMetadata {
	return adapter.RuleSetMetadata{}
}

func (f *fakeRuleSet) ExtractIPSet() []*netipx.IPSet {
	return nil
}

func (f *fakeRuleSet) IncRef() {}

func (f *fakeRuleSet) DecRef() {}

func (f *fakeRuleSet) Cleanup() {}

func (f *fakeRuleSet) RegisterCallback(adapter.RuleSetUpdateCallback) *list.Element[adapter.RuleSetUpdateCallback] {
	return nil
}

func (f *fakeRuleSet) UnregisterCallback(*list.Element[adapter.RuleSetUpdateCallback]) {}

func (f *fakeRuleSet) Close() error {
	return nil
}

func (f *fakeRuleSet) Match(*adapter.InboundContext) bool {
	return f.matched
}

func (f *fakeRuleSet) String() string {
	return "fake-rule-set"
}

type fakeRuleItem struct {
	matched bool
}

func (f *fakeRuleItem) Match(*adapter.InboundContext) bool {
	return f.matched
}

func (f *fakeRuleItem) String() string {
	return "fake-rule-item"
}

func newRuleSetOnlyRule(ruleSetMatched bool, invert bool) *DefaultRule {
	ruleSetItem := &RuleSetItem{
		setList: []adapter.RuleSet{&fakeRuleSet{matched: ruleSetMatched}},
	}
	return &DefaultRule{
		abstractDefaultRule: abstractDefaultRule{
			items:    []RuleItem{ruleSetItem},
			allItems: []RuleItem{ruleSetItem},
			invert:   invert,
		},
	}
}

func newSingleItemRule(matched bool) *DefaultRule {
	item := &fakeRuleItem{matched: matched}
	return &DefaultRule{
		abstractDefaultRule: abstractDefaultRule{
			items:    []RuleItem{item},
			allItems: []RuleItem{item},
		},
	}
}

func TestAbstractDefaultRule_RuleSetOnly_InvertFalse(t *testing.T) {
	t.Parallel()
	require.True(t, newRuleSetOnlyRule(true, false).Match(&adapter.InboundContext{}))
	require.False(t, newRuleSetOnlyRule(false, false).Match(&adapter.InboundContext{}))
}

func TestAbstractDefaultRule_RuleSetOnly_InvertTrue(t *testing.T) {
	t.Parallel()
	require.False(t, newRuleSetOnlyRule(true, true).Match(&adapter.InboundContext{}))
	require.True(t, newRuleSetOnlyRule(false, true).Match(&adapter.InboundContext{}))
}

func TestAbstractLogicalRule_And_WithRuleSetInvert(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name          string
		aMatched      bool
		ruleSetBMatch bool
		expected      bool
	}{
		{
			name:          "A true B true",
			aMatched:      true,
			ruleSetBMatch: true,
			expected:      false,
		},
		{
			name:          "A true B false",
			aMatched:      true,
			ruleSetBMatch: false,
			expected:      true,
		},
		{
			name:          "A false B true",
			aMatched:      false,
			ruleSetBMatch: true,
			expected:      false,
		},
		{
			name:          "A false B false",
			aMatched:      false,
			ruleSetBMatch: false,
			expected:      false,
		},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			logicalRule := &abstractLogicalRule{
				mode: C.LogicalTypeAnd,
				rules: []adapter.HeadlessRule{
					newSingleItemRule(testCase.aMatched),
					newRuleSetOnlyRule(testCase.ruleSetBMatch, true),
				},
			}
			require.Equal(t, testCase.expected, logicalRule.Match(&adapter.InboundContext{}))
		})
	}
}
