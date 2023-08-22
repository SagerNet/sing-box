package option

import (
	"reflect"

	"github.com/sagernet/sing-box/common/json"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

type _SniffOverrideRule struct {
	Type           string                   `json:"type,omitempty"`
	DefaultOptions DefaultSniffOverrideRule `json:"-"`
	LogicalOptions LogicalSniffOverrideRule `json:"-"`
}

type SniffOverrideRule _SniffOverrideRule

func (r SniffOverrideRule) MarshalJSON() ([]byte, error) {
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
	return MarshallObjects((_SniffOverrideRule)(r), v)
}

func (r *SniffOverrideRule) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_SniffOverrideRule)(r))
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
	err = UnmarshallExcluded(bytes, (*_SniffOverrideRule)(r), v)
	if err != nil {
		return E.Cause(err, "route rule")
	}
	return nil
}

type DefaultSniffOverrideRule struct {
	IPVersion       int              `json:"ip_version,omitempty"`
	Network         Listable[string] `json:"network,omitempty"`
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
	ProcessPath     Listable[string] `json:"process_path,omitempty"`
	PackageName     Listable[string] `json:"package_name,omitempty"`
	User            Listable[string] `json:"user,omitempty"`
	UserID          Listable[int32]  `json:"user_id,omitempty"`
	ClashMode       string           `json:"clash_mode,omitempty"`
	Invert          bool             `json:"invert,omitempty"`
}

func (r DefaultSniffOverrideRule) IsValid() bool {
	var defaultValue DefaultSniffOverrideRule
	defaultValue.Invert = r.Invert
	return !reflect.DeepEqual(r, defaultValue)
}

type LogicalSniffOverrideRule struct {
	Mode   string                     `json:"mode"`
	Rules  []DefaultSniffOverrideRule `json:"rules,omitempty"`
	Invert bool                       `json:"invert,omitempty"`
}

func (r LogicalSniffOverrideRule) IsValid() bool {
	return len(r.Rules) > 0 && common.All(r.Rules, DefaultSniffOverrideRule.IsValid)
}
