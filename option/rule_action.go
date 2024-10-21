package option

import (
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
)

type _RuleAction struct {
	Action         string              `json:"action,omitempty"`
	RouteOptions   RouteActionOptions  `json:"-"`
	RejectOptions  RejectActionOptions `json:"-"`
	SniffOptions   RouteActionSniff    `json:"-"`
	ResolveOptions RouteActionResolve  `json:"-"`
}

type RuleAction _RuleAction

func (r RuleAction) MarshalJSON() ([]byte, error) {
	var v any
	switch r.Action {
	case C.RuleActionTypeRoute:
		r.Action = ""
		v = r.RouteOptions
	case C.RuleActionTypeReturn:
		v = nil
	case C.RuleActionTypeReject:
		v = r.RejectOptions
	case C.RuleActionTypeHijackDNS:
		v = nil
	case C.RuleActionTypeSniff:
		v = r.SniffOptions
	case C.RuleActionTypeResolve:
		v = r.ResolveOptions
	default:
		return nil, E.New("unknown rule action: " + r.Action)
	}
	if v == nil {
		return MarshallObjects((_RuleAction)(r))
	}
	return MarshallObjects((_RuleAction)(r), v)
}

func (r *RuleAction) UnmarshalJSON(data []byte) error {
	err := json.Unmarshal(data, (*_RuleAction)(r))
	if err != nil {
		return err
	}
	var v any
	switch r.Action {
	case "", C.RuleActionTypeRoute:
		r.Action = C.RuleActionTypeRoute
		v = &r.RouteOptions
	case C.RuleActionTypeReturn:
		v = nil
	case C.RuleActionTypeReject:
		v = &r.RejectOptions
	case C.RuleActionTypeHijackDNS:
		v = nil
	case C.RuleActionTypeSniff:
		v = &r.SniffOptions
	case C.RuleActionTypeResolve:
		v = &r.ResolveOptions
	default:
		return E.New("unknown rule action: " + r.Action)
	}
	if v == nil {
		// check unknown fields
		return json.UnmarshalDisallowUnknownFields(data, &_RuleAction{})
	}
	return UnmarshallExcluded(data, (*_RuleAction)(r), v)
}

type _DNSRuleAction struct {
	Action         string                `json:"action,omitempty"`
	RouteOptions   DNSRouteActionOptions `json:"-"`
	RejectOptions  RejectActionOptions   `json:"-"`
	SniffOptions   RouteActionSniff      `json:"-"`
	ResolveOptions RouteActionResolve    `json:"-"`
}

type DNSRuleAction _DNSRuleAction

func (r DNSRuleAction) MarshalJSON() ([]byte, error) {
	var v any
	switch r.Action {
	case C.RuleActionTypeRoute:
		r.Action = ""
		v = r.RouteOptions
	case C.RuleActionTypeReturn:
		v = nil
	case C.RuleActionTypeReject:
		v = r.RejectOptions
	default:
		return nil, E.New("unknown DNS rule action: " + r.Action)
	}
	if v == nil {
		return MarshallObjects((_DNSRuleAction)(r))
	}
	return MarshallObjects((_DNSRuleAction)(r), v)
}

func (r *DNSRuleAction) UnmarshalJSON(data []byte) error {
	err := json.Unmarshal(data, (*_DNSRuleAction)(r))
	if err != nil {
		return err
	}
	var v any
	switch r.Action {
	case "", C.RuleActionTypeRoute:
		r.Action = C.RuleActionTypeRoute
		v = &r.RouteOptions
	case C.RuleActionTypeReturn:
		v = nil
	case C.RuleActionTypeReject:
		v = &r.RejectOptions
	default:
		return E.New("unknown DNS rule action: " + r.Action)
	}
	if v == nil {
		// check unknown fields
		return json.UnmarshalDisallowUnknownFields(data, &_DNSRuleAction{})
	}
	return UnmarshallExcluded(data, (*_DNSRuleAction)(r), v)
}

type RouteActionOptions struct {
	Outbound                  string `json:"outbound"`
	UDPDisableDomainUnmapping bool   `json:"udp_disable_domain_unmapping,omitempty"`
}

type DNSRouteActionOptions struct {
	Server       string      `json:"server"`
	DisableCache bool        `json:"disable_cache,omitempty"`
	RewriteTTL   *uint32     `json:"rewrite_ttl,omitempty"`
	ClientSubnet *AddrPrefix `json:"client_subnet,omitempty"`
}

type RejectActionOptions struct {
	Method RejectMethod `json:"method,omitempty"`
}

type RejectMethod string

func (m *RejectMethod) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*string)(m))
	if err != nil {
		return err
	}
	switch *m {
	case C.RuleActionRejectMethodDefault, C.RuleActionRejectMethodPortUnreachable, C.RuleActionRejectMethodDrop:
		return nil
	default:
		return E.New("unknown reject method: " + *m)
	}
}

type RouteActionSniff struct {
	Sniffer Listable[string] `json:"sniffer,omitempty"`
	Timeout Duration         `json:"timeout,omitempty"`
}

type RouteActionResolve struct {
	Strategy DomainStrategy `json:"strategy,omitempty"`
	Server   string         `json:"server,omitempty"`
}
