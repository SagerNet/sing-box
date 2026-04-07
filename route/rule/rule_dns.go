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

	"github.com/miekg/dns"
)

func NewDNSRule(ctx context.Context, logger log.ContextLogger, options option.DNSRule, checkServer bool, legacyDNSMode bool) (adapter.DNSRule, error) {
	switch options.Type {
	case "", C.RuleTypeDefault:
		if !options.DefaultOptions.IsValid() {
			return nil, E.New("missing conditions")
		}
		if !checkServer && options.DefaultOptions.Action == C.RuleActionTypeEvaluate {
			return nil, E.New(options.DefaultOptions.Action, " is only allowed on top-level DNS rules")
		}
		err := validateDNSRuleAction(options.DefaultOptions.DNSRuleAction)
		if err != nil {
			return nil, err
		}
		switch options.DefaultOptions.Action {
		case "", C.RuleActionTypeRoute, C.RuleActionTypeEvaluate:
			if options.DefaultOptions.RouteOptions.Server == "" && checkServer {
				return nil, E.New("missing server field")
			}
		}
		return NewDefaultDNSRule(ctx, logger, options.DefaultOptions, legacyDNSMode)
	case C.RuleTypeLogical:
		if !options.LogicalOptions.IsValid() {
			return nil, E.New("missing conditions")
		}
		if !checkServer && options.LogicalOptions.Action == C.RuleActionTypeEvaluate {
			return nil, E.New(options.LogicalOptions.Action, " is only allowed on top-level DNS rules")
		}
		err := validateDNSRuleAction(options.LogicalOptions.DNSRuleAction)
		if err != nil {
			return nil, err
		}
		switch options.LogicalOptions.Action {
		case "", C.RuleActionTypeRoute, C.RuleActionTypeEvaluate:
			if options.LogicalOptions.RouteOptions.Server == "" && checkServer {
				return nil, E.New("missing server field")
			}
		}
		return NewLogicalDNSRule(ctx, logger, options.LogicalOptions, legacyDNSMode)
	default:
		return nil, E.New("unknown rule type: ", options.Type)
	}
}

func validateDNSRuleAction(action option.DNSRuleAction) error {
	if action.Action == C.RuleActionTypeReject && action.RejectOptions.Method == C.RuleActionRejectMethodReply {
		return E.New("reject method `reply` is not supported for DNS rules")
	}
	return nil
}

var _ adapter.DNSRule = (*DefaultDNSRule)(nil)

type DefaultDNSRule struct {
	abstractDefaultRule
	matchResponse bool
}

func (r *DefaultDNSRule) matchStates(metadata *adapter.InboundContext) ruleMatchStateSet {
	return r.abstractDefaultRule.matchStates(metadata)
}

func NewDefaultDNSRule(ctx context.Context, logger log.ContextLogger, options option.DefaultDNSRule, legacyDNSMode bool) (*DefaultDNSRule, error) {
	rule := &DefaultDNSRule{
		abstractDefaultRule: abstractDefaultRule{
			invert: options.Invert,
			action: NewDNSRuleAction(logger, options.DNSRuleAction),
		},
		matchResponse: options.MatchResponse,
	}
	if len(options.Inbound) > 0 {
		item := NewInboundRule(options.Inbound)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	router := service.FromContext[adapter.Router](ctx)
	networkManager := service.FromContext[adapter.NetworkManager](ctx)
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
	if len(options.QueryType) > 0 {
		item := NewQueryTypeItem(options.QueryType)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
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
			return nil, E.Cause(err, "domain_regex")
		}
		rule.destinationAddressItems = append(rule.destinationAddressItems, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.Geosite) > 0 { //nolint:staticcheck
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
	if len(options.IPCIDR) > 0 {
		item, err := NewIPCIDRItem(false, options.IPCIDR)
		if err != nil {
			return nil, E.Cause(err, "ip_cidr")
		}
		rule.destinationIPCIDRItems = append(rule.destinationIPCIDRItems, item)
		rule.allItems = append(rule.allItems, item)
	}
	if options.SourceIPIsPrivate {
		item := NewIPIsPrivateItem(true)
		rule.sourceAddressItems = append(rule.sourceAddressItems, item)
		rule.allItems = append(rule.allItems, item)
	}
	if options.IPIsPrivate {
		item := NewIPIsPrivateItem(false)
		rule.destinationIPCIDRItems = append(rule.destinationIPCIDRItems, item)
		rule.allItems = append(rule.allItems, item)
	}
	if options.IPAcceptAny { //nolint:staticcheck
		if legacyDNSMode {
			deprecated.Report(ctx, deprecated.OptionIPAcceptAny)
		} else {
			return nil, E.New(deprecated.OptionIPAcceptAny.MessageWithLink())
		}
		item := NewIPAcceptAnyItem()
		rule.destinationIPCIDRItems = append(rule.destinationIPCIDRItems, item)
		rule.allItems = append(rule.allItems, item)
	}
	if options.ResponseRcode != nil {
		item := NewDNSResponseRCodeItem(int(*options.ResponseRcode))
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.ResponseAnswer) > 0 {
		item := NewDNSResponseRecordItem("response_answer", options.ResponseAnswer, dnsResponseAnswers)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.ResponseNs) > 0 {
		item := NewDNSResponseRecordItem("response_ns", options.ResponseNs, dnsResponseNS)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.ResponseExtra) > 0 {
		item := NewDNSResponseRecordItem("response_extra", options.ResponseExtra, dnsResponseExtra)
		rule.items = append(rule.items, item)
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
	if len(options.Outbound) > 0 {
		item := NewOutboundRule(ctx, options.Outbound)
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
	if len(options.SourceMACAddress) > 0 {
		item := NewSourceMACAddressItem(options.SourceMACAddress)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if len(options.SourceHostname) > 0 {
		item := NewSourceHostnameItem(options.SourceHostname)
		rule.items = append(rule.items, item)
		rule.allItems = append(rule.allItems, item)
	}
	if options.RuleSetIPCIDRAcceptEmpty { //nolint:staticcheck
		if legacyDNSMode {
			deprecated.Report(ctx, deprecated.OptionRuleSetIPCIDRAcceptEmpty)
		} else {
			return nil, E.New(deprecated.OptionRuleSetIPCIDRAcceptEmpty.MessageWithLink())
		}
	}
	if len(options.RuleSet) > 0 {
		//nolint:staticcheck
		if options.Deprecated_RulesetIPCIDRMatchSource {
			return nil, E.New("rule_set_ipcidr_match_source is deprecated in sing-box 1.10.0 and removed in sing-box 1.11.0")
		}
		var matchSource bool
		if options.RuleSetIPCIDRMatchSource {
			matchSource = true
		}
		item := NewRuleSetItem(router, options.RuleSet, matchSource, options.RuleSetIPCIDRAcceptEmpty) //nolint:staticcheck
		rule.ruleSetItem = item
		rule.allItems = append(rule.allItems, item)
	}
	return rule, nil
}

func (r *DefaultDNSRule) Action() adapter.RuleAction {
	return r.action
}

func (r *DefaultDNSRule) WithAddressLimit() bool {
	if len(r.destinationIPCIDRItems) > 0 {
		return true
	}
	if r.ruleSetItem != nil {
		ruleSet, isRuleSet := r.ruleSetItem.(*RuleSetItem)
		if isRuleSet && ruleSet.ContainsDestinationIPCIDRRule() {
			return true
		}
	}
	return false
}

func (r *DefaultDNSRule) Match(metadata *adapter.InboundContext) bool {
	return !r.matchStatesForMatch(metadata).isEmpty()
}

func (r *DefaultDNSRule) LegacyPreMatch(metadata *adapter.InboundContext) bool {
	if r.matchResponse {
		return false
	}
	metadata.IgnoreDestinationIPCIDRMatch = true
	defer func() { metadata.IgnoreDestinationIPCIDRMatch = false }()
	return !r.abstractDefaultRule.matchStates(metadata).isEmpty()
}

func (r *DefaultDNSRule) matchStatesForMatch(metadata *adapter.InboundContext) ruleMatchStateSet {
	if r.matchResponse {
		if metadata.DNSResponse == nil {
			return r.abstractDefaultRule.invertedFailure(0)
		}
		matchMetadata := *metadata
		matchMetadata.DestinationAddressMatchFromResponse = true
		return r.abstractDefaultRule.matchStates(&matchMetadata)
	}
	return r.abstractDefaultRule.matchStates(metadata)
}

func (r *DefaultDNSRule) MatchAddressLimit(metadata *adapter.InboundContext, response *dns.Msg) bool {
	matchMetadata := *metadata
	matchMetadata.DNSResponse = response
	matchMetadata.DestinationAddressMatchFromResponse = true
	return !r.abstractDefaultRule.matchStates(&matchMetadata).isEmpty()
}

var _ adapter.DNSRule = (*LogicalDNSRule)(nil)

type LogicalDNSRule struct {
	abstractLogicalRule
}

func (r *LogicalDNSRule) matchStates(metadata *adapter.InboundContext) ruleMatchStateSet {
	return r.abstractLogicalRule.matchStates(metadata)
}

func matchDNSHeadlessRuleStatesForMatch(rule adapter.HeadlessRule, metadata *adapter.InboundContext) ruleMatchStateSet {
	switch typedRule := rule.(type) {
	case *DefaultDNSRule:
		return typedRule.matchStatesForMatch(metadata)
	case *LogicalDNSRule:
		return typedRule.matchStatesForMatch(metadata)
	default:
		return matchHeadlessRuleStates(typedRule, metadata)
	}
}

func (r *LogicalDNSRule) matchStatesForMatch(metadata *adapter.InboundContext) ruleMatchStateSet {
	var stateSet ruleMatchStateSet
	if r.mode == C.LogicalTypeAnd {
		stateSet = emptyRuleMatchState()
		for _, rule := range r.rules {
			nestedMetadata := *metadata
			nestedMetadata.ResetRuleCache()
			nestedStateSet := matchDNSHeadlessRuleStatesForMatch(rule, &nestedMetadata)
			if nestedStateSet.isEmpty() {
				if r.invert {
					return emptyRuleMatchState()
				}
				return 0
			}
			stateSet = stateSet.combine(nestedStateSet)
		}
	} else {
		for _, rule := range r.rules {
			nestedMetadata := *metadata
			nestedMetadata.ResetRuleCache()
			stateSet = stateSet.merge(matchDNSHeadlessRuleStatesForMatch(rule, &nestedMetadata))
		}
		if stateSet.isEmpty() {
			if r.invert {
				return emptyRuleMatchState()
			}
			return 0
		}
	}
	if r.invert {
		return 0
	}
	return stateSet
}

func NewLogicalDNSRule(ctx context.Context, logger log.ContextLogger, options option.LogicalDNSRule, legacyDNSMode bool) (*LogicalDNSRule, error) {
	r := &LogicalDNSRule{
		abstractLogicalRule: abstractLogicalRule{
			rules:  make([]adapter.HeadlessRule, len(options.Rules)),
			invert: options.Invert,
			action: NewDNSRuleAction(logger, options.DNSRuleAction),
		},
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
		err := validateNoNestedDNSRuleActions(subRule, true)
		if err != nil {
			return nil, E.Cause(err, "sub rule[", i, "]")
		}
		rule, err := NewDNSRule(ctx, logger, subRule, false, legacyDNSMode)
		if err != nil {
			return nil, E.Cause(err, "sub rule[", i, "]")
		}
		r.rules[i] = rule
	}
	return r, nil
}

func (r *LogicalDNSRule) Action() adapter.RuleAction {
	return r.action
}

func (r *LogicalDNSRule) WithAddressLimit() bool {
	for _, rawRule := range r.rules {
		switch rule := rawRule.(type) {
		case *DefaultDNSRule:
			if rule.WithAddressLimit() {
				return true
			}
		case *LogicalDNSRule:
			if rule.WithAddressLimit() {
				return true
			}
		}
	}
	return false
}

func (r *LogicalDNSRule) Match(metadata *adapter.InboundContext) bool {
	return !r.matchStatesForMatch(metadata).isEmpty()
}

func (r *LogicalDNSRule) LegacyPreMatch(metadata *adapter.InboundContext) bool {
	metadata.IgnoreDestinationIPCIDRMatch = true
	defer func() { metadata.IgnoreDestinationIPCIDRMatch = false }()
	return !r.abstractLogicalRule.matchStates(metadata).isEmpty()
}

func (r *LogicalDNSRule) MatchAddressLimit(metadata *adapter.InboundContext, response *dns.Msg) bool {
	matchMetadata := *metadata
	matchMetadata.DNSResponse = response
	matchMetadata.DestinationAddressMatchFromResponse = true
	return !r.abstractLogicalRule.matchStates(&matchMetadata).isEmpty()
}
