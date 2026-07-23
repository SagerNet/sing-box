package rule

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
)

var _ RuleItem = (*RuleSetItem)(nil)

type RuleSetItem struct {
	router            adapter.Router
	tagList           []string
	setList           []adapter.RuleSet
	ipCidrMatchSource bool
	ipCidrAcceptEmpty bool
}

func NewRuleSetItem(router adapter.Router, tagList []string, ipCIDRMatchSource bool, ipCidrAcceptEmpty bool) *RuleSetItem {
	return &RuleSetItem{
		router:            router,
		tagList:           tagList,
		ipCidrMatchSource: ipCIDRMatchSource,
		ipCidrAcceptEmpty: ipCidrAcceptEmpty,
	}
}

func (r *RuleSetItem) Start() error {
	_ = r.Close()
	for _, tag := range r.tagList {
		ruleSet, loaded := r.router.RuleSet(tag)
		if !loaded {
			_ = r.Close()
			return E.New("rule-set not found: ", tag)
		}
		ruleSet.IncRef()
		r.setList = append(r.setList, ruleSet)
	}
	return nil
}

func (r *RuleSetItem) Close() error {
	for _, ruleSet := range r.setList {
		ruleSet.DecRef()
	}
	clear(r.setList)
	r.setList = nil
	return nil
}

func (r *RuleSetItem) Match(metadata *adapter.InboundContext) bool {
	for _, ruleSet := range r.setList {
		nestedMetadata := r.nestedMetadata(metadata)
		if ruleSet.Match(&nestedMetadata) {
			return true
		}
	}
	return false
}

func (r *RuleSetItem) matchWithOuterGroups(metadata *adapter.InboundContext, outerGroups ruleGroupMatch) bool {
	outerDone := outerGroups.done()
	for _, ruleSet := range r.setList {
		nestedMetadata := r.nestedMetadata(metadata)
		if provider, isProvider := ruleSet.(mergeableRuleProvider); isProvider {
			branch := provider.mergeableRule()
			if branch != nil {
				branchGroups, branchMatched := branch.evaluateForMerge(&nestedMetadata)
				if branchMatched && outerGroups.mergeWith(branchGroups).done() {
					return true
				}
				continue
			}
		}
		if outerDone && ruleSet.Match(&nestedMetadata) {
			return true
		}
	}
	return false
}

func (r *RuleSetItem) nestedMetadata(metadata *adapter.InboundContext) adapter.InboundContext {
	nestedMetadata := *metadata
	nestedMetadata.ResetRuleMatchCache()
	nestedMetadata.IPCIDRMatchSource = r.ipCidrMatchSource
	nestedMetadata.IPCIDRAcceptEmpty = r.ipCidrAcceptEmpty
	return nestedMetadata
}

type mergeableRuleProvider interface {
	mergeableRule() *DefaultHeadlessRule
}

func mergeableRuleIn(rules []adapter.HeadlessRule) *DefaultHeadlessRule {
	if len(rules) != 1 {
		return nil
	}
	rule, isDefault := rules[0].(*DefaultHeadlessRule)
	if !isDefault || rule.invert || rule.ruleSetItem != nil {
		return nil
	}
	return rule
}

func matchAnyHeadlessRule(rules []adapter.HeadlessRule, metadata *adapter.InboundContext) bool {
	for _, rule := range rules {
		nestedMetadata := *metadata
		nestedMetadata.ResetRuleMatchCache()
		if rule.Match(&nestedMetadata) {
			return true
		}
	}
	return false
}

func (r *RuleSetItem) ContainsDestinationIPCIDRRule() bool {
	if r.ipCidrMatchSource {
		return false
	}
	return common.Any(r.setList, func(ruleSet adapter.RuleSet) bool {
		return ruleSet.Metadata().ContainsIPCIDRRule
	})
}

func (r *RuleSetItem) String() string {
	if len(r.tagList) == 1 {
		return F.ToString("rule_set=", r.tagList[0])
	} else {
		return F.ToString("rule_set=[", strings.Join(r.tagList, " "), "]")
	}
}
