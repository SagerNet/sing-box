package option

import (
	"github.com/sagernet/sing-box/common/json"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

type RouteOptions struct {
	GeoIP               *GeoIPOptions   `json:"geoip,omitempty"`
	Geosite             *GeositeOptions `json:"geosite,omitempty"`
	Rules               []Rule          `json:"rules,omitempty"`
	Final               string          `json:"final,omitempty"`
	FindProcess         bool            `json:"find_process,omitempty"`
	AutoDetectInterface bool            `json:"auto_detect_interface,omitempty"`
	DefaultInterface    string          `json:"default_interface,omitempty"`
	DefaultMark         int             `json:"default_mark,omitempty"`
}

func (o RouteOptions) Equals(other RouteOptions) bool {
	return common.ComparablePtrEquals(o.GeoIP, other.GeoIP) &&
		common.ComparablePtrEquals(o.Geosite, other.Geosite) &&
		common.SliceEquals(o.Rules, other.Rules) &&
		o.Final == other.Final &&
		o.FindProcess == other.FindProcess &&
		o.AutoDetectInterface == other.AutoDetectInterface &&
		o.DefaultInterface == other.DefaultInterface &&
		o.DefaultMark == other.DefaultMark
}

type GeoIPOptions struct {
	Path           string `json:"path,omitempty"`
	DownloadURL    string `json:"download_url,omitempty"`
	DownloadDetour string `json:"download_detour,omitempty"`
}

type GeositeOptions struct {
	Path           string `json:"path,omitempty"`
	DownloadURL    string `json:"download_url,omitempty"`
	DownloadDetour string `json:"download_detour,omitempty"`
}

type _Rule struct {
	Type           string      `json:"type,omitempty"`
	DefaultOptions DefaultRule `json:"-"`
	LogicalOptions LogicalRule `json:"-"`
}

type Rule _Rule

func (r Rule) Equals(other Rule) bool {
	return r.Type == other.Type &&
		r.DefaultOptions.Equals(other.DefaultOptions) &&
		r.LogicalOptions.Equals(other.LogicalOptions)
}

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
	return MarshallObjects((_Rule)(r), v)
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
	err = UnmarshallExcluded(bytes, (*_Rule)(r), v)
	if err != nil {
		return E.Cause(err, "route rule")
	}
	return nil
}

type DefaultRule struct {
	Inbound         Listable[string] `json:"inbound,omitempty"`
	IPVersion       int              `json:"ip_version,omitempty"`
	Network         string           `json:"network,omitempty"`
	AuthUser        Listable[string] `json:"auth_user,omitempty"`
	Protocol        Listable[string] `json:"protocol,omitempty"`
	Domain          Listable[string] `json:"domain,omitempty"`
	DomainSuffix    Listable[string] `json:"domain_suffix,omitempty"`
	DomainKeyword   Listable[string] `json:"domain_keyword,omitempty"`
	DomainRegex     Listable[string] `json:"domain_regex,omitempty"`
	Geosite         Listable[string] `json:"geosite,omitempty"`
	SourceGeoIP     Listable[string] `json:"source_geoip,omitempty"`
	GeoIP           Listable[string] `json:"geoip,omitempty"`
	SourceIPCIDR    Listable[string] `json:"source_ip_cidr,omitempty"`
	IPCIDR          Listable[string] `json:"ip_cidr,omitempty"`
	SourcePort      Listable[uint16] `json:"source_port,omitempty"`
	SourcePortRange Listable[string] `json:"source_port_range,omitempty"`
	Port            Listable[uint16] `json:"port,omitempty"`
	PortRange       Listable[string] `json:"port_range,omitempty"`
	ProcessName     Listable[string] `json:"process_name,omitempty"`
	PackageName     Listable[string] `json:"package_name,omitempty"`
	User            Listable[string] `json:"user,omitempty"`
	UserID          Listable[int32]  `json:"user_id,omitempty"`
	Invert          bool             `json:"invert,omitempty"`
	Outbound        string           `json:"outbound,omitempty"`
}

func (r DefaultRule) IsValid() bool {
	var defaultValue DefaultRule
	defaultValue.Outbound = r.Outbound
	return !r.Equals(defaultValue)
}

func (r DefaultRule) Equals(other DefaultRule) bool {
	return common.ComparableSliceEquals(r.Inbound, other.Inbound) &&
		r.IPVersion == other.IPVersion &&
		r.Network == other.Network &&
		common.ComparableSliceEquals(r.User, other.User) &&
		common.ComparableSliceEquals(r.Protocol, other.Protocol) &&
		common.ComparableSliceEquals(r.Domain, other.Domain) &&
		common.ComparableSliceEquals(r.DomainSuffix, other.DomainSuffix) &&
		common.ComparableSliceEquals(r.DomainKeyword, other.DomainKeyword) &&
		common.ComparableSliceEquals(r.DomainRegex, other.DomainRegex) &&
		common.ComparableSliceEquals(r.Geosite, other.Geosite) &&
		common.ComparableSliceEquals(r.SourceGeoIP, other.SourceGeoIP) &&
		common.ComparableSliceEquals(r.GeoIP, other.GeoIP) &&
		common.ComparableSliceEquals(r.SourceIPCIDR, other.SourceIPCIDR) &&
		common.ComparableSliceEquals(r.IPCIDR, other.IPCIDR) &&
		common.ComparableSliceEquals(r.SourcePort, other.SourcePort) &&
		common.ComparableSliceEquals(r.SourcePortRange, other.SourcePortRange) &&
		common.ComparableSliceEquals(r.Port, other.Port) &&
		common.ComparableSliceEquals(r.PortRange, other.PortRange) &&
		common.ComparableSliceEquals(r.ProcessName, other.ProcessName) &&
		common.ComparableSliceEquals(r.PackageName, other.PackageName) &&
		common.ComparableSliceEquals(r.User, other.User) &&
		common.ComparableSliceEquals(r.UserID, other.UserID) &&
		r.Invert == other.Invert &&
		r.Outbound == other.Outbound
}

type LogicalRule struct {
	Mode     string        `json:"mode"`
	Rules    []DefaultRule `json:"rules,omitempty"`
	Invert   bool          `json:"invert,omitempty"`
	Outbound string        `json:"outbound,omitempty"`
}

func (r LogicalRule) IsValid() bool {
	return len(r.Rules) > 0 && common.All(r.Rules, DefaultRule.IsValid)
}

func (r LogicalRule) Equals(other LogicalRule) bool {
	return r.Mode == other.Mode &&
		common.SliceEquals(r.Rules, other.Rules) &&
		r.Invert == other.Invert &&
		r.Outbound == other.Outbound
}
