package rule

import "github.com/sagernet/sing-box/adapter"

type ruleMatchState uint8

const (
	ruleMatchSourceAddress ruleMatchState = 1 << iota
	ruleMatchSourcePort
	ruleMatchDestinationAddress
	ruleMatchDestinationPort
)

type ruleMatchStateSet uint16

func singleRuleMatchState(state ruleMatchState) ruleMatchStateSet {
	return 1 << state
}

func emptyRuleMatchState() ruleMatchStateSet {
	return singleRuleMatchState(0)
}

func (s ruleMatchStateSet) isEmpty() bool {
	return s == 0
}

func (s ruleMatchStateSet) contains(state ruleMatchState) bool {
	return s&(1<<state) != 0
}

func (s ruleMatchStateSet) add(state ruleMatchState) ruleMatchStateSet {
	return s | singleRuleMatchState(state)
}

func (s ruleMatchStateSet) merge(other ruleMatchStateSet) ruleMatchStateSet {
	return s | other
}

func (s ruleMatchStateSet) combine(other ruleMatchStateSet) ruleMatchStateSet {
	if s.isEmpty() || other.isEmpty() {
		return 0
	}
	var combined ruleMatchStateSet
	for left := range ruleMatchState(16) {
		if !s.contains(left) {
			continue
		}
		for right := range ruleMatchState(16) {
			if !other.contains(right) {
				continue
			}
			combined = combined.add(left | right)
		}
	}
	return combined
}

func (s ruleMatchStateSet) withBase(base ruleMatchState) ruleMatchStateSet {
	if s.isEmpty() {
		return 0
	}
	var withBase ruleMatchStateSet
	for state := range ruleMatchState(16) {
		if !s.contains(state) {
			continue
		}
		withBase = withBase.add(state | base)
	}
	return withBase
}

func (s ruleMatchStateSet) filter(allowed func(ruleMatchState) bool) ruleMatchStateSet {
	var filtered ruleMatchStateSet
	for state := range ruleMatchState(16) {
		if !s.contains(state) {
			continue
		}
		if allowed(state) {
			filtered = filtered.add(state)
		}
	}
	return filtered
}

type ruleStateMatcher interface {
	matchStates(metadata *adapter.InboundContext) ruleMatchStateSet
}

type ruleStateMatcherWithBase interface {
	matchStatesWithBase(metadata *adapter.InboundContext, base ruleMatchState) ruleMatchStateSet
}

func matchHeadlessRuleStatesWithBase(rule adapter.HeadlessRule, metadata *adapter.InboundContext, base ruleMatchState) ruleMatchStateSet {
	if matcher, isStateMatcher := rule.(ruleStateMatcherWithBase); isStateMatcher {
		return matcher.matchStatesWithBase(metadata, base)
	}
	if matcher, isStateMatcher := rule.(ruleStateMatcher); isStateMatcher {
		stateSet := matcher.matchStates(metadata).withBase(base)
		metadata.DefinitiveMatchStates = uint16(ruleMatchStateSet(metadata.DefinitiveMatchStates).withBase(base))
		return stateSet
	}
	if rule.Match(metadata) {
		stateSet := emptyRuleMatchState().withBase(base)
		metadata.DefinitiveMatchStates = uint16(stateSet)
		return stateSet
	}
	metadata.DefinitiveMatchStates = 0
	return 0
}

func matchRuleItemStatesWithBase(item RuleItem, metadata *adapter.InboundContext, base ruleMatchState) ruleMatchStateSet {
	if matcher, isStateMatcher := item.(ruleStateMatcherWithBase); isStateMatcher {
		return matcher.matchStatesWithBase(metadata, base)
	}
	if matcher, isStateMatcher := item.(ruleStateMatcher); isStateMatcher {
		stateSet := matcher.matchStates(metadata).withBase(base)
		metadata.DefinitiveMatchStates = uint16(ruleMatchStateSet(metadata.DefinitiveMatchStates).withBase(base))
		return stateSet
	}
	if item.Match(metadata) {
		stateSet := emptyRuleMatchState().withBase(base)
		metadata.DefinitiveMatchStates = uint16(stateSet)
		return stateSet
	}
	metadata.DefinitiveMatchStates = 0
	return 0
}
