package rule

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/deprecated"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/service"
)

func NewRule(ctx context.Context, logger log.ContextLogger, options option.Rule, checkOutbound bool) (adapter.Rule, error) {
	switch options.Type {
	case "", C.RuleTypeDefault:
		if !options.DefaultOptions.IsValid() {
			return nil, E.New("missing conditions")
		}
		switch options.DefaultOptions.Action {
		case "", C.RuleActionTypeRoute:
			if options.DefaultOptions.RouteOptions.Outbound == "" && checkOutbound {
				return nil, E.New("missing outbound field")
			}
		}
		return NewDefaultRule(ctx, logger, options.DefaultOptions)
	case C.RuleTypeLogical:
		if !options.LogicalOptions.IsValid() {
			return nil, E.New("missing conditions")
		}
		switch options.LogicalOptions.Action {
		case "", C.RuleActionTypeRoute:
			if options.LogicalOptions.RouteOptions.Outbound == "" && checkOutbound {
				return nil, E.New("missing outbound field")
			}
		}
		return NewLogicalRule(ctx, logger, options.LogicalOptions)
	default:
		return nil, E.New("unknown rule type: ", options.Type)
	}
}

var _ adapter.Rule = (*DefaultRule)(nil)

type DefaultRule struct {
	abstractDefaultRule
}

type RuleItem interface {
	Match(metadata *adapter.InboundContext) bool
	String() string
}

func NewDefaultRule(ctx context.Context, logger log.ContextLogger, options option.DefaultRule) (*DefaultRule, error) {
	action, err := NewRuleAction(ctx, logger, options.RuleAction)
	if err != nil {
		return nil, E.Cause(err, "action")
	}
	rule := &DefaultRule{
		abstractDefaultRule{
			invert: options.Invert,
			action: action,
		},
	}
	router := service.FromContext[adapter.Router](ctx)
	networkManager := service.FromContext[adapter.NetworkManager](ctx)
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
	if len(options.Network) > 0 {
		item := NewNetworkItem(options.Network)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
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
	if len(options.Client) > 0 {
		item := NewClientItem(options.Client)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.Domain) > 0 || len(options.DomainSuffix) > 0 {
		item, err := NewDomainItem(options.Domain, options.DomainSuffix)
		if err != nil {
			return nil, err
		}
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
			return nil, err
		}
		rule.destinationAddressItems = append(rule.destinationAddressItems, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.Geosite) > 0 {
		return nil, E.New("geosite database is deprecated in sing-box 1.8.0 and removed in sing-box 1.12.0")
	}
	if len(options.SourceGeoIP) > 0 {
		return nil, E.New("geoip database is deprecated in sing-box 1.8.0 and removed in sing-box 1.12.0")
	}
	if len(options.GeoIP) > 0 {
		return nil, E.New("geoip database is deprecated in sing-box 1.8.0 and removed in sing-box 1.12.0")
	}
	if len(options.SourceIPCIDR) > 0 {
		item, err := NewIPCIDRItem(true, options.SourceIPCIDR)
		if err != nil {
			return nil, E.Cause(err, "source_ip_cidr")
		}
		rule.sourceAddressItems = append(rule.sourceAddressItems, item)
		rule.allItems = append(rule.allItems, item)
	}
	if options.SourceIPIsPrivate {
		item := NewIPIsPrivateItem(true)
		rule.sourceAddressItems = append(rule.sourceAddressItems, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.IPCIDR) > 0 {
		item, err := NewIPCIDRItem(false, options.IPCIDR)
		if err != nil {
			return nil, E.Cause(err, "ipcidr")
		}
		rule.destinationIPCIDRItems = append(rule.destinationIPCIDRItems, item)
		rule.allItems = append(rule.allItems, item)
	}
	if options.IPIsPrivate {
		item := NewIPIsPrivateItem(false)
		rule.destinationIPCIDRItems = append(rule.destinationIPCIDRItems, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.SourcePort) > 0 {
		item := NewPortItem(true, options.SourcePort)
		rule.sourcePortItems = append(rule.sourcePortItems, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.SourcePortRange) > 0 {
		item, err := NewPortRangeItem(true, options.SourcePortRange)
		if err != nil {
			return nil, E.Cause(err, "source_port_range")
		}
		rule.sourcePortItems = append(rule.sourcePortItems, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.Port) > 0 {
		item := NewPortItem(false, options.Port)
		rule.destinationPortItems = append(rule.destinationPortItems, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.PortRange) > 0 {
		item, err := NewPortRangeItem(false, options.PortRange)
		if err != nil {
			return nil, E.Cause(err, "port_range")
		}
		rule.destinationPortItems = append(rule.destinationPortItems, item)
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
	if len(options.ProcessPathRegex) > 0 {
		item, err := NewProcessPathRegexItem(options.ProcessPathRegex)
		if err != nil {
			return nil, E.Cause(err, "process_path_regex")
		}
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
	if options.ClashMode != "" {
		item := NewClashModeItem(ctx, options.ClashMode)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.NetworkType) > 0 {
		item := NewNetworkTypeItem(networkManager, common.Map(options.NetworkType, option.InterfaceType.Build))
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if options.NetworkIsExpensive {
		item := NewNetworkIsExpensiveItem(networkManager)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if options.NetworkIsConstrained {
		item := NewNetworkIsConstrainedItem(networkManager)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.WIFISSID) > 0 {
		item := NewWIFISSIDItem(networkManager, options.WIFISSID)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.WIFIBSSID) > 0 {
		item := NewWIFIBSSIDItem(networkManager, options.WIFIBSSID)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if options.InterfaceAddress != nil && options.InterfaceAddress.Size() > 0 {
		item := NewInterfaceAddressItem(networkManager, options.InterfaceAddress)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if options.NetworkInterfaceAddress != nil && options.NetworkInterfaceAddress.Size() > 0 {
		item := NewNetworkInterfaceAddressItem(networkManager, options.NetworkInterfaceAddress)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.DefaultInterfaceAddress) > 0 {
		item := NewDefaultInterfaceAddressItem(networkManager, options.DefaultInterfaceAddress)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.PreferredBy) > 0 {
		item := NewPreferredByItem(ctx, options.PreferredBy)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.RuleSet) > 0 {
		var matchSource bool
		if options.RuleSetIPCIDRMatchSource {
			matchSource = true
		} else
		//nolint:staticcheck
		if options.Deprecated_RulesetIPCIDRMatchSource {
			matchSource = true
			deprecated.Report(ctx, deprecated.OptionBadMatchSource)
		}
		item := NewRuleSetItem(router, options.RuleSet, matchSource, false)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	return rule, nil
}

var _ adapter.Rule = (*LogicalRule)(nil)

type LogicalRule struct {
	abstractLogicalRule
}

func NewLogicalRule(ctx context.Context, logger log.ContextLogger, options option.LogicalRule) (*LogicalRule, error) {
	action, err := NewRuleAction(ctx, logger, options.RuleAction)
	if err != nil {
		return nil, E.Cause(err, "action")
	}
	rule := &LogicalRule{
		abstractLogicalRule{
			rules:  make([]adapter.HeadlessRule, len(options.Rules)),
			invert: options.Invert,
			action: action,
		},
	}
	switch options.Mode {
	case C.LogicalTypeAnd:
		rule.mode = C.LogicalTypeAnd
	case C.LogicalTypeOr:
		rule.mode = C.LogicalTypeOr
	default:
		return nil, E.New("unknown logical mode: ", options.Mode)
	}
	for i, subOptions := range options.Rules {
		subRule, err := NewRule(ctx, logger, subOptions, false)
		if err != nil {
			return nil, E.Cause(err, "sub rule[", i, "]")
		}
		rule.rules[i] = subRule
	}
	return rule, nil
}
