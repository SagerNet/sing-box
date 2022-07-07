package route

import (
	"strings"

	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

func NewRule(router adapter.Router, logger log.Logger, options option.Rule) (adapter.Rule, error) {
	if common.IsEmptyByEquals(options) {
		return nil, E.New("empty rule config")
	}
	switch options.Type {
	case "", C.RuleTypeDefault:
		if !options.DefaultOptions.IsValid() {
			return nil, E.New("missing conditions")
		}
		if options.DefaultOptions.Outbound == "" {
			return nil, E.New("missing outbound field")
		}
		return NewDefaultRule(router, logger, options.DefaultOptions)
	case C.RuleTypeLogical:
		if !options.LogicalOptions.IsValid() {
			return nil, E.New("missing conditions")
		}
		if options.LogicalOptions.Outbound == "" {
			return nil, E.New("missing outbound field")
		}
		return NewLogicalRule(router, logger, options.LogicalOptions)
	default:
		return nil, E.New("unknown rule type: ", options.Type)
	}
}

var _ adapter.Rule = (*DefaultRule)(nil)

type DefaultRule struct {
	items                   []RuleItem
	sourceAddressItems      []RuleItem
	destinationAddressItems []RuleItem
	allItems                []RuleItem
	outbound                string
}

type RuleItem interface {
	Match(metadata *adapter.InboundContext) bool
	String() string
}

func NewDefaultRule(router adapter.Router, logger log.Logger, options option.DefaultRule) (*DefaultRule, error) {
	rule := &DefaultRule{
		outbound: options.Outbound,
	}
	if len(options.Inbound) > 0 {
		item := NewInboundRule(options.Inbound)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if options.IPVersion > 0 {
		switch options.IPVersion {
		case 4, 6:
			item := NewIPVersionItem(options.IPVersion == 6)
			rule.items = append(rule.items, item)
			rule.allItems = append(rule.allItems, item)
		default:
			return nil, E.New("invalid ip version: ", options.IPVersion)
		}
	}
	if options.Network != "" {
		switch options.Network {
		case C.NetworkTCP, C.NetworkUDP:
			item := NewNetworkItem(options.Network)
			rule.items = append(rule.items, item)
			rule.allItems = append(rule.allItems, item)
		default:
			return nil, E.New("invalid network: ", options.Network)
		}
	}
	if len(options.Protocol) > 0 {
		item := NewProtocolItem(options.Protocol)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.Domain) > 0 || len(options.DomainSuffix) > 0 {
		item := NewDomainItem(options.Domain, options.DomainSuffix)
		rule.destinationAddressItems = append(rule.destinationAddressItems, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.DomainKeyword) > 0 {
		item := NewDomainKeywordItem(options.DomainKeyword)
		rule.destinationAddressItems = append(rule.destinationAddressItems, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.DomainRegex) > 0 {
		item, err := NewDomainRegexItem(options.DomainRegex)
		if err != nil {
			return nil, E.Cause(err, "domain_regex")
		}
		rule.destinationAddressItems = append(rule.destinationAddressItems, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.Geosite) > 0 {
		item := NewGeositeItem(router, logger, options.Geosite)
		rule.destinationAddressItems = append(rule.destinationAddressItems, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.SourceGeoIP) > 0 {
		item := NewGeoIPItem(router, logger, true, options.SourceGeoIP)
		rule.sourceAddressItems = append(rule.sourceAddressItems, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.GeoIP) > 0 {
		item := NewGeoIPItem(router, logger, false, options.GeoIP)
		rule.destinationAddressItems = append(rule.destinationAddressItems, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.SourceIPCIDR) > 0 {
		item, err := NewIPCIDRItem(true, options.SourceIPCIDR)
		if err != nil {
			return nil, E.Cause(err, "source_ipcidr")
		}
		rule.sourceAddressItems = append(rule.sourceAddressItems, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.IPCIDR) > 0 {
		item, err := NewIPCIDRItem(false, options.IPCIDR)
		if err != nil {
			return nil, E.Cause(err, "ipcidr")
		}
		rule.destinationAddressItems = append(rule.destinationAddressItems, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.SourcePort) > 0 {
		item := NewPortItem(true, options.SourcePort)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.Port) > 0 {
		item := NewPortItem(false, options.Port)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	return rule, nil
}

func (r *DefaultRule) Start() error {
	for _, item := range r.allItems {
		err := common.Start(item)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *DefaultRule) Close() error {
	for _, item := range r.allItems {
		err := common.Close(item)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *DefaultRule) UpdateGeosite() error {
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

func (r *DefaultRule) Match(metadata *adapter.InboundContext) bool {
	for _, item := range r.items {
		if !item.Match(metadata) {
			return false
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
			return false
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
			return false
		}
	}

	return true
}

func (r *DefaultRule) Outbound() string {
	return r.outbound
}

func (r *DefaultRule) String() string {
	return strings.Join(common.Map(r.allItems, F.ToString0[RuleItem]), " ")
}

var _ adapter.Rule = (*LogicalRule)(nil)

type LogicalRule struct {
	mode     string
	rules    []*DefaultRule
	outbound string
}

func (r *LogicalRule) UpdateGeosite() error {
	for _, rule := range r.rules {
		err := rule.UpdateGeosite()
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *LogicalRule) Start() error {
	for _, rule := range r.rules {
		err := rule.Start()
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *LogicalRule) Close() error {
	for _, rule := range r.rules {
		err := rule.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func NewLogicalRule(router adapter.Router, logger log.Logger, options option.LogicalRule) (*LogicalRule, error) {
	r := &LogicalRule{
		rules:    make([]*DefaultRule, len(options.Rules)),
		outbound: options.Outbound,
	}
	switch options.Mode {
	case C.LogicalTypeAnd:
		r.mode = C.LogicalTypeAnd
	case C.LogicalTypeOr:
		r.mode = C.LogicalTypeOr
	default:
		return nil, E.New("unknown logical mode: ", options.Mode)
	}
	for i, subRule := range options.Rules {
		rule, err := NewDefaultRule(router, logger, subRule)
		if err != nil {
			return nil, E.Cause(err, "sub rule[", i, "]")
		}
		r.rules[i] = rule
	}
	return r, nil
}

func (r *LogicalRule) Match(metadata *adapter.InboundContext) bool {
	if r.mode == C.LogicalTypeAnd {
		return common.All(r.rules, func(it *DefaultRule) bool {
			return it.Match(metadata)
		})
	} else {
		return common.Any(r.rules, func(it *DefaultRule) bool {
			return it.Match(metadata)
		})
	}
}

func (r *LogicalRule) Outbound() string {
	return r.outbound
}

func (r *LogicalRule) String() string {
	var op string
	switch r.mode {
	case C.LogicalTypeAnd:
		op = "&&"
	case C.LogicalTypeOr:
		op = "||"
	}
	return "logical(" + strings.Join(common.Map(r.rules, F.ToString0[*DefaultRule]), " "+op+" ") + ")"
}
