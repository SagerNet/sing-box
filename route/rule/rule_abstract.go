package rule

import (
	"io"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	F "github.com/sagernet/sing/common/format"
)

type abstractDefaultRule struct {
	items                   []RuleItem
	sourceAddressItems      []RuleItem
	sourcePortItems         []RuleItem
	destinationAddressItems []RuleItem
	destinationIPCIDRItems  []RuleItem
	destinationPortItems    []RuleItem
	allItems                []RuleItem
	ruleSetItem             RuleItem
	invert                  bool
	action                  adapter.RuleAction
}

func (r *abstractDefaultRule) Type() string {
	return C.RuleTypeDefault
}

func (r *abstractDefaultRule) Start() error {
	for _, item := range r.allItems {
		if starter, isStarter := item.(interface {
			Start() error
		}); isStarter {
			err := starter.Start()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *abstractDefaultRule) Close() error {
	for _, item := range r.allItems {
		err := common.Close(item)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *abstractDefaultRule) Match(metadata *adapter.InboundContext) bool {
	return !r.matchStates(metadata).isEmpty()
}

func (r *abstractDefaultRule) destinationIPCIDRMatchesSource(metadata *adapter.InboundContext) bool {
	return metadata.IPCIDRMatchSource && len(r.destinationIPCIDRItems) > 0
}

func (r *abstractDefaultRule) destinationIPCIDRMatchesDestination(metadata *adapter.InboundContext) bool {
	return !metadata.IgnoreDestinationIPCIDRMatch && !metadata.IPCIDRMatchSource && len(r.destinationIPCIDRItems) > 0
}

func (r *abstractDefaultRule) requiresSourceAddressMatch(metadata *adapter.InboundContext) bool {
	return len(r.sourceAddressItems) > 0 || r.destinationIPCIDRMatchesSource(metadata)
}

func (r *abstractDefaultRule) requiresDestinationAddressMatch(metadata *adapter.InboundContext) bool {
	return len(r.destinationAddressItems) > 0 || r.destinationIPCIDRMatchesDestination(metadata)
}

func (r *abstractDefaultRule) matchStates(metadata *adapter.InboundContext) ruleMatchStateSet {
	return r.matchStatesWithBase(metadata, 0)
}

func (r *abstractDefaultRule) matchStatesWithBase(metadata *adapter.InboundContext, inheritedBase ruleMatchState) ruleMatchStateSet {
	if len(r.allItems) == 0 {
		stateSet := emptyRuleMatchState().withBase(inheritedBase)
		metadata.DefinitiveMatchStates = uint16(stateSet)
		return stateSet
	}
	evaluationBase := inheritedBase
	if r.invert {
		evaluationBase = 0
	}
	baseState := evaluationBase
	if len(r.sourceAddressItems) > 0 {
		if matchAnyItem(r.sourceAddressItems, metadata) {
			baseState |= ruleMatchSourceAddress
		}
	}
	if r.destinationIPCIDRMatchesSource(metadata) && !baseState.has(ruleMatchSourceAddress) {
		if matchAnyItem(r.destinationIPCIDRItems, metadata) {
			baseState |= ruleMatchSourceAddress
		}
	}
	if len(r.sourcePortItems) > 0 {
		if matchAnyItem(r.sourcePortItems, metadata) {
			baseState |= ruleMatchSourcePort
		}
	}
	if len(r.destinationAddressItems) > 0 {
		if matchAnyItem(r.destinationAddressItems, metadata) {
			baseState |= ruleMatchDestinationAddress
		}
	}
	if r.destinationIPCIDRMatchesDestination(metadata) && !baseState.has(ruleMatchDestinationAddress) {
		if matchAnyItem(r.destinationIPCIDRItems, metadata) {
			baseState |= ruleMatchDestinationAddress
		}
	}
	if len(r.destinationPortItems) > 0 {
		if matchAnyItem(r.destinationPortItems, metadata) {
			baseState |= ruleMatchDestinationPort
		}
	}
	var deferredGroups ruleMatchState
	if metadata.IgnoreDestinationIPCIDRMatch && !metadata.IPCIDRMatchSource && len(r.destinationIPCIDRItems) > 0 && len(r.destinationAddressItems) == 0 {
		deferredGroups |= ruleMatchDestinationAddress
	}
	for _, item := range r.items {
		if !item.Match(metadata) {
			return r.invertedFailure(metadata, inheritedBase)
		}
	}
	var stateSet, definitiveStateSet ruleMatchStateSet
	if r.ruleSetItem != nil {
		stateSet = matchRuleItemStatesWithBase(r.ruleSetItem, metadata, baseState)
		definitiveStateSet = ruleMatchStateSet(metadata.DefinitiveMatchStates)
	} else {
		stateSet = singleRuleMatchState(baseState)
		definitiveStateSet = stateSet
	}
	if deferredGroups != 0 {
		definitiveStateSet = definitiveStateSet.filter(func(state ruleMatchState) bool {
			return deferredGroups&^state == 0
		})
	}
	stateFilter := func(state ruleMatchState) bool {
		if r.requiresSourceAddressMatch(metadata) && !state.has(ruleMatchSourceAddress) {
			return false
		}
		if len(r.sourcePortItems) > 0 && !state.has(ruleMatchSourcePort) {
			return false
		}
		if r.requiresDestinationAddressMatch(metadata) && !state.has(ruleMatchDestinationAddress) {
			return false
		}
		if len(r.destinationPortItems) > 0 && !state.has(ruleMatchDestinationPort) {
			return false
		}
		return true
	}
	stateSet = stateSet.filter(stateFilter)
	definitiveStateSet = definitiveStateSet.filter(stateFilter)
	if stateSet.isEmpty() {
		return r.invertedFailure(metadata, inheritedBase)
	}
	if r.invert {
		metadata.DefinitiveMatchStates = 0
		if definitiveStateSet.isEmpty() {
			// DNS pre-lookup defers destination address-limit checks until the response phase.
			return emptyRuleMatchState().withBase(inheritedBase)
		}
		return 0
	}
	metadata.DefinitiveMatchStates = uint16(definitiveStateSet)
	return stateSet
}

func (r *abstractDefaultRule) invertedFailure(metadata *adapter.InboundContext, base ruleMatchState) ruleMatchStateSet {
	if r.invert {
		stateSet := emptyRuleMatchState().withBase(base)
		metadata.DefinitiveMatchStates = uint16(stateSet)
		return stateSet
	}
	metadata.DefinitiveMatchStates = 0
	return 0
}

func (r *abstractDefaultRule) Action() adapter.RuleAction {
	return r.action
}

func (r *abstractDefaultRule) String() string {
	if !r.invert {
		return strings.Join(F.MapToString(r.allItems), " ")
	} else {
		return "!(" + strings.Join(F.MapToString(r.allItems), " ") + ")"
	}
}

type abstractLogicalRule struct {
	rules  []adapter.HeadlessRule
	mode   string
	invert bool
	action adapter.RuleAction
}

func (r *abstractLogicalRule) Type() string {
	return C.RuleTypeLogical
}

func (r *abstractLogicalRule) Start() error {
	for _, rule := range common.FilterIsInstance(r.rules, func(it adapter.HeadlessRule) (interface {
		Start() error
	}, bool,
	) {
		rule, loaded := it.(interface {
			Start() error
		})
		return rule, loaded
	}) {
		err := rule.Start()
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *abstractLogicalRule) Close() error {
	for _, rule := range common.FilterIsInstance(r.rules, func(it adapter.HeadlessRule) (io.Closer, bool) {
		rule, loaded := it.(io.Closer)
		return rule, loaded
	}) {
		err := rule.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *abstractLogicalRule) Match(metadata *adapter.InboundContext) bool {
	return !r.matchStates(metadata).isEmpty()
}

func (r *abstractLogicalRule) matchStates(metadata *adapter.InboundContext) ruleMatchStateSet {
	return r.matchStatesWithBase(metadata, 0)
}

func (r *abstractLogicalRule) matchStatesWithBase(metadata *adapter.InboundContext, base ruleMatchState) ruleMatchStateSet {
	evaluationBase := base
	if r.invert {
		evaluationBase = 0
	}
	var stateSet, definitiveStateSet ruleMatchStateSet
	if r.mode == C.LogicalTypeAnd {
		stateSet = emptyRuleMatchState().withBase(evaluationBase)
		definitiveStateSet = stateSet
		for _, rule := range r.rules {
			nestedMetadata := *metadata
			nestedMetadata.ResetRuleCache()
			nestedStateSet := matchHeadlessRuleStatesWithBase(rule, &nestedMetadata, evaluationBase)
			if nestedStateSet.isEmpty() {
				if r.invert {
					invertedStateSet := emptyRuleMatchState().withBase(base)
					metadata.DefinitiveMatchStates = uint16(invertedStateSet)
					return invertedStateSet
				}
				metadata.DefinitiveMatchStates = 0
				return 0
			}
			stateSet = stateSet.combine(nestedStateSet)
			definitiveStateSet = definitiveStateSet.combine(ruleMatchStateSet(nestedMetadata.DefinitiveMatchStates))
		}
	} else {
		for _, rule := range r.rules {
			nestedMetadata := *metadata
			nestedMetadata.ResetRuleCache()
			stateSet = stateSet.merge(matchHeadlessRuleStatesWithBase(rule, &nestedMetadata, evaluationBase))
			definitiveStateSet = definitiveStateSet.merge(ruleMatchStateSet(nestedMetadata.DefinitiveMatchStates))
		}
		if stateSet.isEmpty() {
			if r.invert {
				invertedStateSet := emptyRuleMatchState().withBase(base)
				metadata.DefinitiveMatchStates = uint16(invertedStateSet)
				return invertedStateSet
			}
			metadata.DefinitiveMatchStates = 0
			return 0
		}
	}
	if r.invert {
		metadata.DefinitiveMatchStates = 0
		if definitiveStateSet.isEmpty() {
			// DNS pre-lookup defers destination address-limit checks until the response phase.
			return emptyRuleMatchState().withBase(base)
		}
		return 0
	}
	metadata.DefinitiveMatchStates = uint16(definitiveStateSet)
	return stateSet
}

func (r *abstractLogicalRule) Action() adapter.RuleAction {
	return r.action
}

func (r *abstractLogicalRule) String() string {
	var op string
	switch r.mode {
	case C.LogicalTypeAnd:
		op = "&&"
	case C.LogicalTypeOr:
		op = "||"
	}
	if !r.invert {
		return strings.Join(F.MapToString(r.rules), " "+op+" ")
	} else {
		return "!(" + strings.Join(F.MapToString(r.rules), " "+op+" ") + ")"
	}
}

func matchAnyItem(items []RuleItem, metadata *adapter.InboundContext) bool {
	return common.Any(items, func(it RuleItem) bool {
		return it.Match(metadata)
	})
}

func (s ruleMatchState) has(target ruleMatchState) bool {
	return s&target != 0
}
