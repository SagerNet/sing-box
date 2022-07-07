package option

import (
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"

	C "github.com/sagernet/sing-box/constant"

	"github.com/goccy/go-json"
)

type DNSOptions struct {
	Servers  []DNSServerOptions `json:"servers,omitempty"`
	Rules    []DNSRule          `json:"rules,omitempty"`
	Final    string             `json:"final,omitempty"`
	Strategy DomainStrategy     `json:"strategy,omitempty"`
	DNSClientOptions
}

func (o DNSOptions) Equals(other DNSOptions) bool {
	return common.ComparableSliceEquals(o.Servers, other.Servers) &&
		common.SliceEquals(o.Rules, other.Rules) &&
		o.Final == other.Final &&
		o.Strategy == other.Strategy &&
		o.DNSClientOptions == other.DNSClientOptions
}

type DNSClientOptions struct {
	DisableCache  bool `json:"disable_cache,omitempty"`
	DisableExpire bool `json:"disable_expire,omitempty"`
}

type DNSServerOptions struct {
	Tag             string         `json:"tag,omitempty"`
	Address         string         `json:"address"`
	AddressResolver string         `json:"address_resolver,omitempty"`
	AddressStrategy DomainStrategy `json:"address_strategy,omitempty"`
	DialerOptions
}

type _DNSRule struct {
	Type           string         `json:"type,omitempty"`
	DefaultOptions DefaultDNSRule `json:"-"`
	LogicalOptions LogicalDNSRule `json:"-"`
}

type DNSRule _DNSRule

func (r DNSRule) Equals(other DNSRule) bool {
	return r.Type == other.Type &&
		r.DefaultOptions.Equals(other.DefaultOptions) &&
		r.LogicalOptions.Equals(other.LogicalOptions)
}

func (r DNSRule) MarshalJSON() ([]byte, error) {
	var v any
	switch r.Type {
	case C.RuleTypeDefault:
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
	if r.Type == "" {
		r.Type = C.RuleTypeDefault
	}
	var v any
	switch r.Type {
	case C.RuleTypeDefault:
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

type DefaultDNSRule struct {
	Inbound       Listable[string] `json:"inbound,omitempty"`
	Network       string           `json:"network,omitempty"`
	Protocol      Listable[string] `json:"protocol,omitempty"`
	Domain        Listable[string] `json:"domain,omitempty"`
	DomainSuffix  Listable[string] `json:"domain_suffix,omitempty"`
	DomainKeyword Listable[string] `json:"domain_keyword,omitempty"`
	DomainRegex   Listable[string] `json:"domain_regex,omitempty"`
	Geosite       Listable[string] `json:"geosite,omitempty"`
	SourceGeoIP   Listable[string] `json:"source_geoip,omitempty"`
	SourceIPCIDR  Listable[string] `json:"source_ip_cidr,omitempty"`
	SourcePort    Listable[uint16] `json:"source_port,omitempty"`
	Port          Listable[uint16] `json:"port,omitempty"`
	Outbound      Listable[string] `json:"outbound,omitempty"`
	Server        string           `json:"server,omitempty"`
}

func (r DefaultDNSRule) IsValid() bool {
	var defaultValue DefaultDNSRule
	defaultValue.Server = r.Server
	return !r.Equals(defaultValue)
}

func (r DefaultDNSRule) Equals(other DefaultDNSRule) bool {
	return common.ComparableSliceEquals(r.Inbound, other.Inbound) &&
		r.Network == other.Network &&
		common.ComparableSliceEquals(r.Protocol, other.Protocol) &&
		common.ComparableSliceEquals(r.Domain, other.Domain) &&
		common.ComparableSliceEquals(r.DomainSuffix, other.DomainSuffix) &&
		common.ComparableSliceEquals(r.DomainKeyword, other.DomainKeyword) &&
		common.ComparableSliceEquals(r.DomainRegex, other.DomainRegex) &&
		common.ComparableSliceEquals(r.Geosite, other.Geosite) &&
		common.ComparableSliceEquals(r.SourceGeoIP, other.SourceGeoIP) &&
		common.ComparableSliceEquals(r.SourceIPCIDR, other.SourceIPCIDR) &&
		common.ComparableSliceEquals(r.SourcePort, other.SourcePort) &&
		common.ComparableSliceEquals(r.Port, other.Port) &&
		common.ComparableSliceEquals(r.Outbound, other.Outbound) &&
		r.Server == other.Server
}

type LogicalDNSRule struct {
	Mode   string           `json:"mode"`
	Rules  []DefaultDNSRule `json:"rules,omitempty"`
	Server string           `json:"server,omitempty"`
}

func (r LogicalDNSRule) IsValid() bool {
	return len(r.Rules) > 0 && common.All(r.Rules, DefaultDNSRule.IsValid)
}

func (r LogicalDNSRule) Equals(other LogicalDNSRule) bool {
	return r.Mode == other.Mode &&
		common.SliceEquals(r.Rules, other.Rules) &&
		r.Server == other.Server
}
