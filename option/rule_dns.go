package option

import (
	"context"
	"reflect"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
)

type _DNSRule struct {
	Type           string         `json:"type,omitempty"`
	DefaultOptions DefaultDNSRule `json:"-"`
	LogicalOptions LogicalDNSRule `json:"-"`
}

type DNSRule _DNSRule

func (r DNSRule) MarshalJSON() ([]byte, error) {
	var v any
	switch r.Type {
	case C.RuleTypeDefault:
		r.Type = ""
		v = r.DefaultOptions
	case C.RuleTypeLogical:
		v = r.LogicalOptions
	default:
		return nil, E.New("unknown rule type: " + r.Type)
	}
	return badjson.MarshallObjects((_DNSRule)(r), v)
}

func (r *DNSRule) UnmarshalJSONContext(ctx context.Context, bytes []byte) error {
	err := json.Unmarshal(bytes, (*_DNSRule)(r))
	if err != nil {
		return err
	}
	var v any
	switch r.Type {
	case "", C.RuleTypeDefault:
		r.Type = C.RuleTypeDefault
		v = &r.DefaultOptions
	case C.RuleTypeLogical:
		v = &r.LogicalOptions
	default:
		return E.New("unknown rule type: " + r.Type)
	}
	err = badjson.UnmarshallExcludedContext(ctx, bytes, (*_DNSRule)(r), v)
	if err != nil {
		return err
	}
	return nil
}

func (r DNSRule) IsValid() bool {
	switch r.Type {
	case C.RuleTypeDefault:
		return r.DefaultOptions.IsValid()
	case C.RuleTypeLogical:
		return r.LogicalOptions.IsValid()
	default:
		panic("unknown DNS rule type: " + r.Type)
	}
}

type RawDefaultDNSRule struct {
	Inbound                  badoption.Listable[string]                                                  `json:"inbound,omitempty"`
	IPVersion                int                                                                         `json:"ip_version,omitempty"`
	QueryType                badoption.Listable[DNSQueryType]                                            `json:"query_type,omitempty"`
	Network                  badoption.Listable[string]                                                  `json:"network,omitempty"`
	AuthUser                 badoption.Listable[string]                                                  `json:"auth_user,omitempty"`
	Protocol                 badoption.Listable[string]                                                  `json:"protocol,omitempty"`
	Domain                   badoption.Listable[string]                                                  `json:"domain,omitempty"`
	DomainSuffix             badoption.Listable[string]                                                  `json:"domain_suffix,omitempty"`
	DomainKeyword            badoption.Listable[string]                                                  `json:"domain_keyword,omitempty"`
	DomainRegex              badoption.Listable[string]                                                  `json:"domain_regex,omitempty"`
	Geosite                  badoption.Listable[string]                                                  `json:"geosite,omitempty"`
	SourceGeoIP              badoption.Listable[string]                                                  `json:"source_geoip,omitempty"`
	GeoIP                    badoption.Listable[string]                                                  `json:"geoip,omitempty"`
	IPCIDR                   badoption.Listable[string]                                                  `json:"ip_cidr,omitempty"`
	IPIsPrivate              bool                                                                        `json:"ip_is_private,omitempty"`
	IPAcceptAny              bool                                                                        `json:"ip_accept_any,omitempty"`
	SourceIPCIDR             badoption.Listable[string]                                                  `json:"source_ip_cidr,omitempty"`
	SourceIPIsPrivate        bool                                                                        `json:"source_ip_is_private,omitempty"`
	SourcePort               badoption.Listable[uint16]                                                  `json:"source_port,omitempty"`
	SourcePortRange          badoption.Listable[string]                                                  `json:"source_port_range,omitempty"`
	Port                     badoption.Listable[uint16]                                                  `json:"port,omitempty"`
	PortRange                badoption.Listable[string]                                                  `json:"port_range,omitempty"`
	ProcessName              badoption.Listable[string]                                                  `json:"process_name,omitempty"`
	ProcessPath              badoption.Listable[string]                                                  `json:"process_path,omitempty"`
	ProcessPathRegex         badoption.Listable[string]                                                  `json:"process_path_regex,omitempty"`
	PackageName              badoption.Listable[string]                                                  `json:"package_name,omitempty"`
	User                     badoption.Listable[string]                                                  `json:"user,omitempty"`
	UserID                   badoption.Listable[int32]                                                   `json:"user_id,omitempty"`
	Outbound                 badoption.Listable[string]                                                  `json:"outbound,omitempty"`
	ClashMode                string                                                                      `json:"clash_mode,omitempty"`
	NetworkType              badoption.Listable[InterfaceType]                                           `json:"network_type,omitempty"`
	NetworkIsExpensive       bool                                                                        `json:"network_is_expensive,omitempty"`
	NetworkIsConstrained     bool                                                                        `json:"network_is_constrained,omitempty"`
	WIFISSID                 badoption.Listable[string]                                                  `json:"wifi_ssid,omitempty"`
	WIFIBSSID                badoption.Listable[string]                                                  `json:"wifi_bssid,omitempty"`
	InterfaceAddress         *badjson.TypedMap[string, badoption.Listable[*badoption.Prefixable]]        `json:"interface_address,omitempty"`
	NetworkInterfaceAddress  *badjson.TypedMap[InterfaceType, badoption.Listable[*badoption.Prefixable]] `json:"network_interface_address,omitempty"`
	DefaultInterfaceAddress  badoption.Listable[*badoption.Prefixable]                                   `json:"default_interface_address,omitempty"`
	RuleSet                  badoption.Listable[string]                                                  `json:"rule_set,omitempty"`
	RuleSetIPCIDRMatchSource bool                                                                        `json:"rule_set_ip_cidr_match_source,omitempty"`
	RuleSetIPCIDRAcceptEmpty bool                                                                        `json:"rule_set_ip_cidr_accept_empty,omitempty"`
	Invert                   bool                                                                        `json:"invert,omitempty"`

	// Deprecated: renamed to rule_set_ip_cidr_match_source
	Deprecated_RulesetIPCIDRMatchSource bool `json:"rule_set_ipcidr_match_source,omitempty"`
}

type DefaultDNSRule struct {
	RawDefaultDNSRule
	DNSRuleAction
}

func (r DefaultDNSRule) MarshalJSON() ([]byte, error) {
	return badjson.MarshallObjects(r.RawDefaultDNSRule, r.DNSRuleAction)
}

func (r *DefaultDNSRule) UnmarshalJSONContext(ctx context.Context, data []byte) error {
	err := json.UnmarshalContext(ctx, data, &r.RawDefaultDNSRule)
	if err != nil {
		return err
	}
	return badjson.UnmarshallExcludedContext(ctx, data, &r.RawDefaultDNSRule, &r.DNSRuleAction)
}

func (r DefaultDNSRule) IsValid() bool {
	var defaultValue DefaultDNSRule
	defaultValue.Invert = r.Invert
	return !reflect.DeepEqual(r, defaultValue)
}

type RawLogicalDNSRule struct {
	Mode   string    `json:"mode"`
	Rules  []DNSRule `json:"rules,omitempty"`
	Invert bool      `json:"invert,omitempty"`
}

type LogicalDNSRule struct {
	RawLogicalDNSRule
	DNSRuleAction
}

func (r LogicalDNSRule) MarshalJSON() ([]byte, error) {
	return badjson.MarshallObjects(r.RawLogicalDNSRule, r.DNSRuleAction)
}

func (r *LogicalDNSRule) UnmarshalJSONContext(ctx context.Context, data []byte) error {
	err := json.Unmarshal(data, &r.RawLogicalDNSRule)
	if err != nil {
		return err
	}
	return badjson.UnmarshallExcludedContext(ctx, data, &r.RawLogicalDNSRule, &r.DNSRuleAction)
}

func (r *LogicalDNSRule) IsValid() bool {
	return len(r.Rules) > 0 && common.All(r.Rules, DNSRule.IsValid)
}
