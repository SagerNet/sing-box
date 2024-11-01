package option

import (
	"reflect"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
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

func (r *DNSRule) UnmarshalJSON(bytes []byte) error {
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
	err = badjson.UnmarshallExcluded(bytes, (*_DNSRule)(r), v)
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
	Inbound                  Listable[string]       `json:"inbound,omitempty"`
	IPVersion                int                    `json:"ip_version,omitempty"`
	QueryType                Listable[DNSQueryType] `json:"query_type,omitempty"`
	Network                  Listable[string]       `json:"network,omitempty"`
	AuthUser                 Listable[string]       `json:"auth_user,omitempty"`
	Protocol                 Listable[string]       `json:"protocol,omitempty"`
	Domain                   Listable[string]       `json:"domain,omitempty"`
	DomainSuffix             Listable[string]       `json:"domain_suffix,omitempty"`
	DomainKeyword            Listable[string]       `json:"domain_keyword,omitempty"`
	DomainRegex              Listable[string]       `json:"domain_regex,omitempty"`
	Geosite                  Listable[string]       `json:"geosite,omitempty"`
	SourceGeoIP              Listable[string]       `json:"source_geoip,omitempty"`
	GeoIP                    Listable[string]       `json:"geoip,omitempty"`
	IPCIDR                   Listable[string]       `json:"ip_cidr,omitempty"`
	IPIsPrivate              bool                   `json:"ip_is_private,omitempty"`
	SourceIPCIDR             Listable[string]       `json:"source_ip_cidr,omitempty"`
	SourceIPIsPrivate        bool                   `json:"source_ip_is_private,omitempty"`
	SourcePort               Listable[uint16]       `json:"source_port,omitempty"`
	SourcePortRange          Listable[string]       `json:"source_port_range,omitempty"`
	Port                     Listable[uint16]       `json:"port,omitempty"`
	PortRange                Listable[string]       `json:"port_range,omitempty"`
	ProcessName              Listable[string]       `json:"process_name,omitempty"`
	ProcessPath              Listable[string]       `json:"process_path,omitempty"`
	ProcessPathRegex         Listable[string]       `json:"process_path_regex,omitempty"`
	PackageName              Listable[string]       `json:"package_name,omitempty"`
	User                     Listable[string]       `json:"user,omitempty"`
	UserID                   Listable[int32]        `json:"user_id,omitempty"`
	Outbound                 Listable[string]       `json:"outbound,omitempty"`
	ClashMode                string                 `json:"clash_mode,omitempty"`
	WIFISSID                 Listable[string]       `json:"wifi_ssid,omitempty"`
	WIFIBSSID                Listable[string]       `json:"wifi_bssid,omitempty"`
	RuleSet                  Listable[string]       `json:"rule_set,omitempty"`
	RuleSetIPCIDRMatchSource bool                   `json:"rule_set_ip_cidr_match_source,omitempty"`
	RuleSetIPCIDRAcceptEmpty bool                   `json:"rule_set_ip_cidr_accept_empty,omitempty"`
	Invert                   bool                   `json:"invert,omitempty"`

	// Deprecated: renamed to rule_set_ip_cidr_match_source
	Deprecated_RulesetIPCIDRMatchSource bool `json:"rule_set_ipcidr_match_source,omitempty"`
}

type DefaultDNSRule struct {
	RawDefaultDNSRule
	DNSRuleAction
}

func (r *DefaultDNSRule) MarshalJSON() ([]byte, error) {
	return badjson.MarshallObjects(r.RawDefaultDNSRule, r.DNSRuleAction)
}

func (r *DefaultDNSRule) UnmarshalJSON(data []byte) error {
	err := json.Unmarshal(data, &r.RawDefaultDNSRule)
	if err != nil {
		return err
	}
	return badjson.UnmarshallExcluded(data, &r.RawDefaultDNSRule, &r.DNSRuleAction)
}

func (r *DefaultDNSRule) IsValid() bool {
	var defaultValue DefaultDNSRule
	defaultValue.Invert = r.Invert
	defaultValue.DNSRuleAction = r.DNSRuleAction
	return !reflect.DeepEqual(r, defaultValue)
}

type _LogicalDNSRule struct {
	Mode   string    `json:"mode"`
	Rules  []DNSRule `json:"rules,omitempty"`
	Invert bool      `json:"invert,omitempty"`
}

type LogicalDNSRule struct {
	_LogicalDNSRule
	DNSRuleAction
}

func (r *LogicalDNSRule) MarshalJSON() ([]byte, error) {
	return badjson.MarshallObjects(r._LogicalDNSRule, r.DNSRuleAction)
}

func (r *LogicalDNSRule) UnmarshalJSON(data []byte) error {
	err := json.Unmarshal(data, &r._LogicalDNSRule)
	if err != nil {
		return err
	}
	return badjson.UnmarshallExcluded(data, &r._LogicalDNSRule, &r.DNSRuleAction)
}

func (r *LogicalDNSRule) IsValid() bool {
	return len(r.Rules) > 0 && common.All(r.Rules, DNSRule.IsValid)
}
