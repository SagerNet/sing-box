package option

import (
	"reflect"

	"github.com/sagernet/sing-box/common/json"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

type _IPRule struct {
	Type           string        `json:"type,omitempty"`
	DefaultOptions DefaultIPRule `json:"-"`
	LogicalOptions LogicalIPRule `json:"-"`
}

type IPRule _IPRule

func (r IPRule) MarshalJSON() ([]byte, error) {
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
	return MarshallObjects((_IPRule)(r), v)
}

func (r *IPRule) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_IPRule)(r))
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
	err = UnmarshallExcluded(bytes, (*_IPRule)(r), v)
	if err != nil {
		return E.Cause(err, "ip route rule")
	}
	return nil
}

type DefaultIPRule struct {
	Inbound         Listable[string] `json:"inbound,omitempty"`
	IPVersion       int              `json:"ip_version,omitempty"`
	Network         Listable[string] `json:"network,omitempty"`
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
	Invert          bool             `json:"invert,omitempty"`
	Action          RouteAction      `json:"action,omitempty"`
	Outbound        string           `json:"outbound,omitempty"`
}

type RouteAction tun.ActionType

func (a RouteAction) MarshalJSON() ([]byte, error) {
	typeName, err := tun.ActionTypeName(tun.ActionType(a))
	if err != nil {
		return nil, err
	}
	return json.Marshal(typeName)
}

func (a *RouteAction) UnmarshalJSON(bytes []byte) error {
	var value string
	err := json.Unmarshal(bytes, &value)
	if err != nil {
		return err
	}
	actionType, err := tun.ParseActionType(value)
	if err != nil {
		return err
	}
	*a = RouteAction(actionType)
	return nil
}

func (r DefaultIPRule) IsValid() bool {
	var defaultValue DefaultIPRule
	defaultValue.Invert = r.Invert
	defaultValue.Outbound = r.Outbound
	return !reflect.DeepEqual(r, defaultValue)
}

type LogicalIPRule struct {
	Mode     string          `json:"mode"`
	Rules    []DefaultIPRule `json:"rules,omitempty"`
	Invert   bool            `json:"invert,omitempty"`
	Action   RouteAction     `json:"action,omitempty"`
	Outbound string          `json:"outbound,omitempty"`
}

func (r LogicalIPRule) IsValid() bool {
	return len(r.Rules) > 0 && common.All(r.Rules, DefaultIPRule.IsValid)
}
