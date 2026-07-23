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
	ruleSetItem             *RuleSetItem
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
	if len(r.allItems) == 0 {
		return true
	}
	matched := r.matchInner(metadata)
	if r.invert {
		if matched && metadata.IgnoreDestinationIPCIDRMatch && !metadata.DidMatch && len(r.destinationIPCIDRItems) > 0 {
			return true
		}
		return !matched
	}
	return matched
}

func (r *abstractDefaultRule) matchInner(metadata *adapter.InboundContext) bool {
	groups := r.evaluateGroups(metadata)
	for _, item := range r.items {
		metadata.DidMatch = true
		if !item.Match(metadata) {
			return false
		}
	}
	if r.ruleSetItem != nil {
		metadata.DidMatch = true
		return r.ruleSetItem.matchWithOuterGroups(metadata, groups)
	}
	return groups.done()
}

func (r *abstractDefaultRule) evaluateForMerge(metadata *adapter.InboundContext) (ruleGroupMatch, bool) {
	groups := r.evaluateGroups(metadata)
	for _, item := range r.items {
		metadata.DidMatch = true
		if !item.Match(metadata) {
			return ruleGroupMatch{}, false
		}
	}
	return groups, true
}

func (r *abstractDefaultRule) destinationIPCIDRMatchesSource(metadata *adapter.InboundContext) bool {
	return !metadata.IgnoreDestinationIPCIDRMatch && metadata.IPCIDRMatchSource && len(r.destinationIPCIDRItems) > 0
}

func (r *abstractDefaultRule) destinationIPCIDRMatchesDestination(metadata *adapter.InboundContext) bool {
	return !metadata.IgnoreDestinationIPCIDRMatch && !metadata.IPCIDRMatchSource && len(r.destinationIPCIDRItems) > 0
}

func (r *abstractDefaultRule) evaluateGroups(metadata *adapter.InboundContext) ruleGroupMatch {
	var groups ruleGroupMatch
	if len(r.sourceAddressItems) > 0 {
		metadata.DidMatch = true
		groups.required |= ruleMatchSourceAddress
		if matchAnyItem(r.sourceAddressItems, metadata) {
			groups.satisfied |= ruleMatchSourceAddress
		}
	}
	if r.destinationIPCIDRMatchesSource(metadata) {
		metadata.DidMatch = true
		groups.required |= ruleMatchSourceAddress
		if !groups.satisfied.has(ruleMatchSourceAddress) && matchAnyItem(r.destinationIPCIDRItems, metadata) {
			groups.satisfied |= ruleMatchSourceAddress
		}
	}
	if len(r.sourcePortItems) > 0 {
		metadata.DidMatch = true
		groups.required |= ruleMatchSourcePort
		if matchAnyItem(r.sourcePortItems, metadata) {
			groups.satisfied |= ruleMatchSourcePort
		}
	}
	if len(r.destinationAddressItems) > 0 {
		metadata.DidMatch = true
		groups.required |= ruleMatchDestinationAddress
		if matchAnyItem(r.destinationAddressItems, metadata) {
			groups.satisfied |= ruleMatchDestinationAddress
		}
	}
	if r.destinationIPCIDRMatchesDestination(metadata) {
		metadata.DidMatch = true
		groups.required |= ruleMatchDestinationAddress
		if !groups.satisfied.has(ruleMatchDestinationAddress) && matchAnyItem(r.destinationIPCIDRItems, metadata) {
			groups.satisfied |= ruleMatchDestinationAddress
		}
	}
	if len(r.destinationPortItems) > 0 {
		metadata.DidMatch = true
		groups.required |= ruleMatchDestinationPort
		if matchAnyItem(r.destinationPortItems, metadata) {
			groups.satisfied |= ruleMatchDestinationPort
		}
	}
	return groups
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
	var matched bool
	if r.mode == C.LogicalTypeAnd {
		matched = true
		for _, rule := range r.rules {
			nestedMetadata := *metadata
			nestedMetadata.ResetRuleCache()
			if !rule.Match(&nestedMetadata) {
				matched = false
				break
			}
		}
	} else {
		for _, rule := range r.rules {
			nestedMetadata := *metadata
			nestedMetadata.ResetRuleCache()
			if rule.Match(&nestedMetadata) {
				matched = true
				break
			}
		}
	}
	if r.invert {
		return !matched
	}
	return matched
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
