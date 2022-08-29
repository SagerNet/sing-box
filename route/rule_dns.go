package route

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	N "github.com/sagernet/sing/common/network"
)

func NewDNSRule(router adapter.Router, logger log.ContextLogger, options option.DNSRule) (adapter.DNSRule, error) {
	switch options.Type {
	case "", C.RuleTypeDefault:
		if !options.DefaultOptions.IsValid() {
			return nil, E.New("missing conditions")
		}
		if options.DefaultOptions.Server == "" {
			return nil, E.New("missing server field")
		}
		return NewDefaultDNSRule(router, logger, options.DefaultOptions)
	case C.RuleTypeLogical:
		if !options.LogicalOptions.IsValid() {
			return nil, E.New("missing conditions")
		}
		if options.LogicalOptions.Server == "" {
			return nil, E.New("missing server field")
		}
		return NewLogicalDNSRule(router, logger, options.LogicalOptions)
	default:
		return nil, E.New("unknown rule type: ", options.Type)
	}
}

var _ adapter.DNSRule = (*DefaultDNSRule)(nil)

type DefaultDNSRule struct {
	items        []RuleItem
	addressItems []RuleItem
	allItems     []RuleItem
	invert       bool
	outbound     string
	disableCache bool
}

func NewDefaultDNSRule(router adapter.Router, logger log.ContextLogger, options option.DefaultDNSRule) (*DefaultDNSRule, error) {
	rule := &DefaultDNSRule{
		invert:       options.Invert,
		outbound:     options.Server,
		disableCache: options.DisableCache,
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
		case N.NetworkTCP, N.NetworkUDP:
			item := NewNetworkItem(options.Network)
			rule.items = append(rule.items, item)
			rule.allItems = append(rule.allItems, item)
		default:
			return nil, E.New("invalid network: ", options.Network)
		}
	}
	if len(options.AuthUser) > 0 {
		item := NewAuthUserItem(options.AuthUser)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.Protocol) > 0 {
		item := NewProtocolItem(options.Protocol)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.Domain) > 0 || len(options.DomainSuffix) > 0 {
		item := NewDomainItem(options.Domain, options.DomainSuffix)
		rule.addressItems = append(rule.addressItems, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.DomainKeyword) > 0 {
		item := NewDomainKeywordItem(options.DomainKeyword)
		rule.addressItems = append(rule.addressItems, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.DomainRegex) > 0 {
		item, err := NewDomainRegexItem(options.DomainRegex)
		if err != nil {
			return nil, E.Cause(err, "domain_regex")
		}
		rule.addressItems = append(rule.addressItems, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.Geosite) > 0 {
		item := NewGeositeItem(router, logger, options.Geosite)
		rule.addressItems = append(rule.addressItems, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.SourceGeoIP) > 0 {
		item := NewGeoIPItem(router, logger, true, options.SourceGeoIP)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.SourceIPCIDR) > 0 {
		item, err := NewIPCIDRItem(true, options.SourceIPCIDR)
		if err != nil {
			return nil, E.Cause(err, "source_ipcidr")
		}
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.SourcePort) > 0 {
		item := NewPortItem(true, options.SourcePort)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.SourcePortRange) > 0 {
		item, err := NewPortRangeItem(true, options.SourcePortRange)
		if err != nil {
			return nil, E.Cause(err, "source_port_range")
		}
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.Port) > 0 {
		item := NewPortItem(false, options.Port)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.PortRange) > 0 {
		item, err := NewPortRangeItem(false, options.PortRange)
		if err != nil {
			return nil, E.Cause(err, "port_range")
		}
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.ProcessName) > 0 {
		item := NewProcessItem(options.ProcessName)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.ProcessPath) > 0 {
		item := NewProcessPathItem(options.ProcessPath)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.PackageName) > 0 {
		item := NewPackageNameItem(options.PackageName)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.User) > 0 {
		item := NewUserItem(options.User)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.UserID) > 0 {
		item := NewUserIDItem(options.UserID)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.Outbound) > 0 {
		item := NewOutboundRule(options.Outbound)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	return rule, nil
}

func (r *DefaultDNSRule) Type() string {
	return C.RuleTypeDefault
}

func (r *DefaultDNSRule) Start() error {
	for _, item := range r.allItems {
		err := common.Start(item)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *DefaultDNSRule) Close() error {
	for _, item := range r.allItems {
		err := common.Close(item)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *DefaultDNSRule) UpdateGeosite() error {
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

func (r *DefaultDNSRule) Match(metadata *adapter.InboundContext) bool {
	for _, item := range r.items {
		if !item.Match(metadata) {
			return r.invert
		}
	}
	if len(r.addressItems) > 0 {
		var addressMatch bool
		for _, item := range r.addressItems {
			if item.Match(metadata) {
				addressMatch = true
				break
			}
		}
		if !addressMatch {
			return r.invert
		}
	}
	return !r.invert
}

func (r *DefaultDNSRule) Outbound() string {
	return r.outbound
}

func (r *DefaultDNSRule) DisableCache() bool {
	return r.disableCache
}

func (r *DefaultDNSRule) String() string {
	return strings.Join(F.MapToString(r.allItems), " ")
}

var _ adapter.DNSRule = (*LogicalDNSRule)(nil)

type LogicalDNSRule struct {
	mode         string
	rules        []*DefaultDNSRule
	invert       bool
	outbound     string
	disableCache bool
}

func NewLogicalDNSRule(router adapter.Router, logger log.ContextLogger, options option.LogicalDNSRule) (*LogicalDNSRule, error) {
	r := &LogicalDNSRule{
		rules:        make([]*DefaultDNSRule, len(options.Rules)),
		invert:       options.Invert,
		outbound:     options.Server,
		disableCache: options.DisableCache,
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
		rule, err := NewDefaultDNSRule(router, logger, subRule)
		if err != nil {
			return nil, E.Cause(err, "sub rule[", i, "]")
		}
		r.rules[i] = rule
	}
	return r, nil
}

func (r *LogicalDNSRule) Type() string {
	return C.RuleTypeLogical
}

func (r *LogicalDNSRule) UpdateGeosite() error {
	for _, rule := range r.rules {
		err := rule.UpdateGeosite()
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *LogicalDNSRule) Start() error {
	for _, rule := range r.rules {
		err := rule.Start()
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *LogicalDNSRule) Close() error {
	for _, rule := range r.rules {
		err := rule.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *LogicalDNSRule) Match(metadata *adapter.InboundContext) bool {
	if r.mode == C.LogicalTypeAnd {
		return common.All(r.rules, func(it *DefaultDNSRule) bool {
			return it.Match(metadata)
		}) != r.invert
	} else {
		return common.Any(r.rules, func(it *DefaultDNSRule) bool {
			return it.Match(metadata)
		}) != r.invert
	}
}

func (r *LogicalDNSRule) Outbound() string {
	return r.outbound
}

func (r *LogicalDNSRule) DisableCache() bool {
	return r.disableCache
}

func (r *LogicalDNSRule) String() string {
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
