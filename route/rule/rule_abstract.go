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
		if !matched {
			metadata.DeferredIPCIDRMatchGroups = 0
			return true
		}
		return metadata.DeferredIPCIDRMatchGroups != 0
	}
	return matched
}

func (r *abstractDefaultRule) matchInner(metadata *adapter.InboundContext) bool {
	groups := r.evaluateGroups(metadata)
	for _, item := range r.items {
		if !item.Match(metadata) {
			return false
		}
	}
	var matched bool
	if r.ruleSetItem != nil {
		matched = r.ruleSetItem.matchWithOuterGroups(metadata, groups)
	} else {
		matched = groups.done()
	}
	if matched {
		metadata.DeferredIPCIDRMatchGroups &^= uint8(groups.satisfied)
	}
	return matched
}

func (r *abstractDefaultRule) evaluateForMerge(metadata *adapter.InboundContext) (ruleGroupMatch, bool) {
	groups := r.evaluateGroups(metadata)
	for _, item := range r.items {
		if !item.Match(metadata) {
			return ruleGroupMatch{}, false
		}
	}
	return groups, true
}

func (r *abstractDefaultRule) destinationIPCIDRMatchesSource(metadata *adapter.InboundContext) bool {
	return metadata.IPCIDRMatchSource && len(r.destinationIPCIDRItems) > 0
}

func (r *abstractDefaultRule) destinationIPCIDRMatchesDestination(metadata *adapter.InboundContext) bool {
	return !metadata.IgnoreDestinationIPCIDRMatch && !metadata.IPCIDRMatchSource && len(r.destinationIPCIDRItems) > 0
}

func (r *abstractDefaultRule) evaluateGroups(metadata *adapter.InboundContext) ruleGroupMatch {
	var groups ruleGroupMatch
	if len(r.sourceAddressItems) > 0 {
		groups.required |= ruleMatchSourceAddress
		if matchAnyItem(r.sourceAddressItems, metadata) {
			groups.satisfied |= ruleMatchSourceAddress
		}
	}
	if r.destinationIPCIDRMatchesSource(metadata) {
		groups.required |= ruleMatchSourceAddress
		if !groups.satisfied.has(ruleMatchSourceAddress) && matchAnyItem(r.destinationIPCIDRItems, metadata) {
			groups.satisfied |= ruleMatchSourceAddress
		}
	}
	if len(r.sourcePortItems) > 0 {
		groups.required |= ruleMatchSourcePort
		if matchAnyItem(r.sourcePortItems, metadata) {
			groups.satisfied |= ruleMatchSourcePort
		}
	}
	if len(r.destinationAddressItems) > 0 {
		groups.required |= ruleMatchDestinationAddress
		if matchAnyItem(r.destinationAddressItems, metadata) {
			groups.satisfied |= ruleMatchDestinationAddress
		}
	}
	if r.destinationIPCIDRMatchesDestination(metadata) {
		groups.required |= ruleMatchDestinationAddress
		if !groups.satisfied.has(ruleMatchDestinationAddress) && matchAnyItem(r.destinationIPCIDRItems, metadata) {
			groups.satisfied |= ruleMatchDestinationAddress
		}
	}
	if len(r.destinationPortItems) > 0 {
		groups.required |= ruleMatchDestinationPort
		if matchAnyItem(r.destinationPortItems, metadata) {
			groups.satisfied |= ruleMatchDestinationPort
		}
	}
	if metadata.IgnoreDestinationIPCIDRMatch && !metadata.IPCIDRMatchSource && len(r.destinationIPCIDRItems) > 0 && len(r.destinationAddressItems) == 0 {
		metadata.DeferredIPCIDRMatchGroups |= uint8(ruleMatchDestinationAddress)
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
	var (
		matched        bool
		deferredGroups uint8
	)
	snapshot := snapshotRuleMatch(metadata)
	if r.mode == C.LogicalTypeAnd {
		matched = true
		for _, rule := range r.rules {
			metadata.ResetRuleCache()
			if !rule.Match(metadata) {
				matched = false
				deferredGroups = 0
				break
			}
			deferredGroups |= metadata.DeferredIPCIDRMatchGroups
		}
	} else {
		for _, rule := range r.rules {
			metadata.ResetRuleCache()
			if rule.Match(metadata) {
				matched = true
				if metadata.DeferredIPCIDRMatchGroups == 0 {
					deferredGroups = 0
					break
				}
				deferredGroups |= metadata.DeferredIPCIDRMatchGroups
			}
		}
	}
	snapshot.restore(metadata)
	if matched {
		metadata.DeferredIPCIDRMatchGroups |= deferredGroups
	}
	if r.invert {
		if !matched {
			return true
		}
		return deferredGroups != 0
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
