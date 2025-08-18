package option

import (
	"reflect"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
)

type _Rule struct {
	Type           string      `json:"type,omitempty"`
	DefaultOptions DefaultRule `json:"-"`
	LogicalOptions LogicalRule `json:"-"`
}

type Rule _Rule

func (r Rule) MarshalJSON() ([]byte, error) {
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
	return badjson.MarshallObjects((_Rule)(r), v)
}

func (r *Rule) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_Rule)(r))
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
	err = badjson.UnmarshallExcluded(bytes, (*_Rule)(r), v)
	if err != nil {
		return err
	}
	return nil
}

func (r Rule) IsValid() bool {
	switch r.Type {
	case C.RuleTypeDefault:
		return r.DefaultOptions.IsValid()
	case C.RuleTypeLogical:
		return r.LogicalOptions.IsValid()
	default:
		panic("unknown rule type: " + r.Type)
	}
}

type RawDefaultRule struct {
	Inbound                  badoption.Listable[string]                                                  `json:"inbound,omitempty"`
	IPVersion                int                                                                         `json:"ip_version,omitempty"`
	Network                  badoption.Listable[string]                                                  `json:"network,omitempty"`
	AuthUser                 badoption.Listable[string]                                                  `json:"auth_user,omitempty"`
	Protocol                 badoption.Listable[string]                                                  `json:"protocol,omitempty"`
	Client                   badoption.Listable[string]                                                  `json:"client,omitempty"`
	Domain                   badoption.Listable[string]                                                  `json:"domain,omitempty"`
	DomainSuffix             badoption.Listable[string]                                                  `json:"domain_suffix,omitempty"`
	DomainKeyword            badoption.Listable[string]                                                  `json:"domain_keyword,omitempty"`
	DomainRegex              badoption.Listable[string]                                                  `json:"domain_regex,omitempty"`
	Geosite                  badoption.Listable[string]                                                  `json:"geosite,omitempty"`
	SourceGeoIP              badoption.Listable[string]                                                  `json:"source_geoip,omitempty"`
	GeoIP                    badoption.Listable[string]                                                  `json:"geoip,omitempty"`
	SourceIPCIDR             badoption.Listable[string]                                                  `json:"source_ip_cidr,omitempty"`
	SourceIPIsPrivate        bool                                                                        `json:"source_ip_is_private,omitempty"`
	IPCIDR                   badoption.Listable[string]                                                  `json:"ip_cidr,omitempty"`
	IPIsPrivate              bool                                                                        `json:"ip_is_private,omitempty"`
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
	ClashMode                string                                                                      `json:"clash_mode,omitempty"`
	NetworkType              badoption.Listable[InterfaceType]                                           `json:"network_type,omitempty"`
	NetworkIsExpensive       bool                                                                        `json:"network_is_expensive,omitempty"`
	NetworkIsConstrained     bool                                                                        `json:"network_is_constrained,omitempty"`
	WIFISSID                 badoption.Listable[string]                                                  `json:"wifi_ssid,omitempty"`
	WIFIBSSID                badoption.Listable[string]                                                  `json:"wifi_bssid,omitempty"`
	InterfaceAddress         *badjson.TypedMap[string, badoption.Listable[*badoption.Prefixable]]        `json:"interface_address,omitempty"`
	NetworkInterfaceAddress  *badjson.TypedMap[InterfaceType, badoption.Listable[*badoption.Prefixable]] `json:"network_interface_address,omitempty"`
	DefaultInterfaceAddress  badoption.Listable[*badoption.Prefixable]                                   `json:"default_interface_address,omitempty"`
	PreferredBy              badoption.Listable[string]                                                  `json:"preferred_by,omitempty"`
	RuleSet                  badoption.Listable[string]                                                  `json:"rule_set,omitempty"`
	RuleSetIPCIDRMatchSource bool                                                                        `json:"rule_set_ip_cidr_match_source,omitempty"`
	Invert                   bool                                                                        `json:"invert,omitempty"`

	// Deprecated: renamed to rule_set_ip_cidr_match_source
	Deprecated_RulesetIPCIDRMatchSource bool `json:"rule_set_ipcidr_match_source,omitempty"`
}

type DefaultRule struct {
	RawDefaultRule
	RuleAction
}

func (r DefaultRule) MarshalJSON() ([]byte, error) {
	return badjson.MarshallObjects(r.RawDefaultRule, r.RuleAction)
}

func (r *DefaultRule) UnmarshalJSON(data []byte) error {
	err := json.Unmarshal(data, &r.RawDefaultRule)
	if err != nil {
		return err
	}
	return badjson.UnmarshallExcluded(data, &r.RawDefaultRule, &r.RuleAction)
}

func (r DefaultRule) IsValid() bool {
	var defaultValue DefaultRule
	defaultValue.Invert = r.Invert
	return !reflect.DeepEqual(r, defaultValue)
}

type RawLogicalRule struct {
	Mode   string `json:"mode"`
	Rules  []Rule `json:"rules,omitempty"`
	Invert bool   `json:"invert,omitempty"`
}

type LogicalRule struct {
	RawLogicalRule
	RuleAction
}

func (r LogicalRule) MarshalJSON() ([]byte, error) {
	return badjson.MarshallObjects(r.RawLogicalRule, r.RuleAction)
}

func (r *LogicalRule) UnmarshalJSON(data []byte) error {
	err := json.Unmarshal(data, &r.RawLogicalRule)
	if err != nil {
		return err
	}
	return badjson.UnmarshallExcluded(data, &r.RawLogicalRule, &r.RuleAction)
}

func (r *LogicalRule) IsValid() bool {
	return len(r.Rules) > 0 && common.All(r.Rules, Rule.IsValid)
}
