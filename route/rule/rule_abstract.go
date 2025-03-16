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
	if len(r.allItems) == 0 {
		return true
	}

	if len(r.sourceAddressItems) > 0 && !metadata.SourceAddressMatch {
		metadata.DidMatch = true
		for _, item := range r.sourceAddressItems {
			if item.Match(metadata) {
				metadata.SourceAddressMatch = true
				break
			}
		}
	}

	if len(r.sourcePortItems) > 0 && !metadata.SourcePortMatch {
		metadata.DidMatch = true
		for _, item := range r.sourcePortItems {
			if item.Match(metadata) {
				metadata.SourcePortMatch = true
				break
			}
		}
	}

	if len(r.destinationAddressItems) > 0 && !metadata.DestinationAddressMatch {
		metadata.DidMatch = true
		for _, item := range r.destinationAddressItems {
			if item.Match(metadata) {
				metadata.DestinationAddressMatch = true
				break
			}
		}
	}

	if !metadata.IgnoreDestinationIPCIDRMatch && len(r.destinationIPCIDRItems) > 0 && !metadata.DestinationAddressMatch {
		metadata.DidMatch = true
		for _, item := range r.destinationIPCIDRItems {
			if item.Match(metadata) {
				metadata.DestinationAddressMatch = true
				break
			}
		}
	}

	if len(r.destinationPortItems) > 0 && !metadata.DestinationPortMatch {
		metadata.DidMatch = true
		for _, item := range r.destinationPortItems {
			if item.Match(metadata) {
				metadata.DestinationPortMatch = true
				break
			}
		}
	}

	for _, item := range r.items {
		if _, isRuleSet := item.(*RuleSetItem); !isRuleSet {
			metadata.DidMatch = true
		}
		if !item.Match(metadata) {
			return r.invert
		}
	}

	if len(r.sourceAddressItems) > 0 && !metadata.SourceAddressMatch {
		return r.invert
	}

	if len(r.sourcePortItems) > 0 && !metadata.SourcePortMatch {
		return r.invert
	}

	if ((!metadata.IgnoreDestinationIPCIDRMatch && len(r.destinationIPCIDRItems) > 0) || len(r.destinationAddressItems) > 0) && !metadata.DestinationAddressMatch {
		return r.invert
	}

	if len(r.destinationPortItems) > 0 && !metadata.DestinationPortMatch {
		return r.invert
	}

	if !metadata.DidMatch {
		return true
	}

	return !r.invert
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
	if r.mode == C.LogicalTypeAnd {
		return common.All(r.rules, func(it adapter.HeadlessRule) bool {
			metadata.ResetRuleCache()
			return it.Match(metadata)
		}) != r.invert
	} else {
		return common.Any(r.rules, func(it adapter.HeadlessRule) bool {
			metadata.ResetRuleCache()
			return it.Match(metadata)
		}) != r.invert
	}
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
