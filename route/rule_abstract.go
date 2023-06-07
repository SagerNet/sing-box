package route

import (
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
	destinationPortItems    []RuleItem
	allItems                []RuleItem
	invert                  bool
	outbound                string
}

func (r *abstractDefaultRule) Type() string {
	return C.RuleTypeDefault
}

func (r *abstractDefaultRule) Start() error {
	for _, item := range r.allItems {
		err := common.Start(item)
		if err != nil {
			return err
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

func (r *abstractDefaultRule) UpdateGeosite() error {
	for _, item := range r.allItems {
		if geositeItem, isSite := item.(*GeositeItem); isSite {
			err := geositeItem.Update()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *abstractDefaultRule) Match(metadata *adapter.InboundContext) bool {
	for _, item := range r.items {
		if !item.Match(metadata) {
			return r.invert
		}
	}

	if len(r.sourceAddressItems) > 0 {
		var sourceAddressMatch bool
		for _, item := range r.sourceAddressItems {
			if item.Match(metadata) {
				sourceAddressMatch = true
				break
			}
		}
		if !sourceAddressMatch {
			return r.invert
		}
	}

	if len(r.sourcePortItems) > 0 {
		var sourcePortMatch bool
		for _, item := range r.sourcePortItems {
			if item.Match(metadata) {
				sourcePortMatch = true
				break
			}
		}
		if !sourcePortMatch {
			return r.invert
		}
	}

	if len(r.destinationAddressItems) > 0 {
		var destinationAddressMatch bool
		for _, item := range r.destinationAddressItems {
			if item.Match(metadata) {
				destinationAddressMatch = true
				break
			}
		}
		if !destinationAddressMatch {
			return r.invert
		}
	}

	if len(r.destinationPortItems) > 0 {
		var destinationPortMatch bool
		for _, item := range r.destinationPortItems {
			if item.Match(metadata) {
				destinationPortMatch = true
				break
			}
		}
		if !destinationPortMatch {
			return r.invert
		}
	}

	return !r.invert
}

func (r *abstractDefaultRule) Outbound() string {
	return r.outbound
}

func (r *abstractDefaultRule) String() string {
	if !r.invert {
		return strings.Join(F.MapToString(r.allItems), " ")
	} else {
		return "!(" + strings.Join(F.MapToString(r.allItems), " ") + ")"
	}
}

type abstractLogicalRule struct {
	rules    []adapter.Rule
	mode     string
	invert   bool
	outbound string
}

func (r *abstractLogicalRule) Type() string {
	return C.RuleTypeLogical
}

func (r *abstractLogicalRule) UpdateGeosite() error {
	for _, rule := range r.rules {
		err := rule.UpdateGeosite()
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *abstractLogicalRule) Start() error {
	for _, rule := range r.rules {
		err := rule.Start()
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *abstractLogicalRule) Close() error {
	for _, rule := range r.rules {
		err := rule.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *abstractLogicalRule) Match(metadata *adapter.InboundContext) bool {
	if r.mode == C.LogicalTypeAnd {
		return common.All(r.rules, func(it adapter.Rule) bool {
			return it.Match(metadata)
		}) != r.invert
	} else {
		return common.Any(r.rules, func(it adapter.Rule) bool {
			return it.Match(metadata)
		}) != r.invert
	}
}

func (r *abstractLogicalRule) Outbound() string {
	return r.outbound
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
