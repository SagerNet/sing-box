package option

import (
	"reflect"

	"github.com/sagernet/sing-box/common/json"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
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
	return MarshallObjects((_DNSRule)(r), v)
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
	err = UnmarshallExcluded(bytes, (*_DNSRule)(r), v)
	if err != nil {
		return E.Cause(err, "dns route rule")
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

type DefaultDNSRule struct {
	Inbound         Listable[string]       `json:"inbound,omitempty"`
	IPVersion       int                    `json:"ip_version,omitempty"`
	QueryType       Listable[DNSQueryType] `json:"query_type,omitempty"`
	Network         Listable[string]       `json:"network,omitempty"`
	AuthUser        Listable[string]       `json:"auth_user,omitempty"`
	Protocol        Listable[string]       `json:"protocol,omitempty"`
	Domain          Listable[string]       `json:"domain,omitempty"`
	DomainSuffix    Listable[string]       `json:"domain_suffix,omitempty"`
	DomainKeyword   Listable[string]       `json:"domain_keyword,omitempty"`
	DomainRegex     Listable[string]       `json:"domain_regex,omitempty"`
	Geosite         Listable[string]       `json:"geosite,omitempty"`
	SourceGeoIP     Listable[string]       `json:"source_geoip,omitempty"`
	SourceIPCIDR    Listable[string]       `json:"source_ip_cidr,omitempty"`
	SourcePort      Listable[uint16]       `json:"source_port,omitempty"`
	SourcePortRange Listable[string]       `json:"source_port_range,omitempty"`
	Port            Listable[uint16]       `json:"port,omitempty"`
	PortRange       Listable[string]       `json:"port_range,omitempty"`
	ProcessName     Listable[string]       `json:"process_name,omitempty"`
	ProcessPath     Listable[string]       `json:"process_path,omitempty"`
	PackageName     Listable[string]       `json:"package_name,omitempty"`
	User            Listable[string]       `json:"user,omitempty"`
	UserID          Listable[int32]        `json:"user_id,omitempty"`
	Outbound        Listable[string]       `json:"outbound,omitempty"`
	ClashMode       string                 `json:"clash_mode,omitempty"`
	WIFISSID        Listable[string]       `json:"wifi_ssid,omitempty"`
	WIFIBSSID       Listable[string]       `json:"wifi_bssid,omitempty"`
	RuleSet         Listable[string]       `json:"rule_set,omitempty"`
	Invert          bool                   `json:"invert,omitempty"`
	Server          string                 `json:"server,omitempty"`
	DisableCache    bool                   `json:"disable_cache,omitempty"`
	RewriteTTL      *uint32                `json:"rewrite_ttl,omitempty"`
}

func (r DefaultDNSRule) IsValid() bool {
	var defaultValue DefaultDNSRule
	defaultValue.Invert = r.Invert
	defaultValue.Server = r.Server
	defaultValue.DisableCache = r.DisableCache
	defaultValue.RewriteTTL = r.RewriteTTL
	return !reflect.DeepEqual(r, defaultValue)
}

type LogicalDNSRule struct {
	Mode         string    `json:"mode"`
	Rules        []DNSRule `json:"rules,omitempty"`
	Invert       bool      `json:"invert,omitempty"`
	Server       string    `json:"server,omitempty"`
	DisableCache bool      `json:"disable_cache,omitempty"`
	RewriteTTL   *uint32   `json:"rewrite_ttl,omitempty"`
}

func (r LogicalDNSRule) IsValid() bool {
	return len(r.Rules) > 0 && common.All(r.Rules, DNSRule.IsValid)
}
