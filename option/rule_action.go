package option

import (
	"context"
	"fmt"
	"net/netip"
	"reflect"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/schema"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
)

type _RuleAction struct {
	Action              string                    `json:"action,omitempty" enum:"route,route-options,direct,bypass,reject,hijack-dns,sniff,resolve"`
	RouteOptions        RouteActionOptions        `json:"-"`
	RouteOptionsOptions RouteOptionsActionOptions `json:"-"`
	DirectOptions       DirectActionOptions       `json:"-"`
	BypassOptions       RouteActionOptions        `json:"-"`
	RejectOptions       RejectActionOptions       `json:"-"`
	SniffOptions        RouteActionSniff          `json:"-"`
	ResolveOptions      RouteActionResolve        `json:"-"`
}

type RuleAction _RuleAction

func (r RuleAction) MarshalJSON() ([]byte, error) {
	if r.Action == "" {
		return json.Marshal(struct{}{})
	}
	var v any
	switch r.Action {
	case C.RuleActionTypeRoute:
		r.Action = ""
		v = r.RouteOptions
	case C.RuleActionTypeRouteOptions:
		v = r.RouteOptionsOptions
	case C.RuleActionTypeDirect:
		v = r.DirectOptions
	case C.RuleActionTypeBypass:
		v = r.BypassOptions
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
		return badjson.MarshallObjects((_RuleAction)(r))
	}
	return badjson.MarshallObjects((_RuleAction)(r), v)
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
	case C.RuleActionTypeRouteOptions:
		v = &r.RouteOptionsOptions
	case C.RuleActionTypeDirect:
		v = &r.DirectOptions
	case C.RuleActionTypeBypass:
		v = &r.BypassOptions
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
	err = badjson.UnmarshallExcluded(data, (*_RuleAction)(r), v)
	if err != nil {
		return err
	}
	return nil
}

type _DNSRuleAction struct {
	Action              string                       `json:"action,omitempty" enum:"route,evaluate,respond,route-options,reject,predefined"`
	Race                bool                         `json:"race,omitempty"`
	RouteOptions        DNSRouteActionOptions        `json:"-"`
	EvaluateOptions     DNSEvaluateActionOptions     `json:"-"`
	RouteOptionsOptions DNSRouteOptionsActionOptions `json:"-"`
	RejectOptions       RejectActionOptions          `json:"-"`
	PredefinedOptions   DNSRouteActionPredefined     `json:"-"`
}

type DNSRuleAction _DNSRuleAction

func (r DNSRuleAction) MarshalJSON() ([]byte, error) {
	if r.Action == "" {
		return json.Marshal(struct{}{})
	}
	var v any
	switch r.Action {
	case C.RuleActionTypeRoute:
		r.Action = ""
		v = r.RouteOptions
	case C.RuleActionTypeEvaluate:
		v = r.EvaluateOptions
	case C.RuleActionTypeRespond:
		v = nil
	case C.RuleActionTypeRouteOptions:
		v = r.RouteOptionsOptions
	case C.RuleActionTypeReject:
		v = r.RejectOptions
	case C.RuleActionTypePredefined:
		v = r.PredefinedOptions
	default:
		return nil, E.New("unknown DNS rule action: " + r.Action)
	}
	if v == nil {
		return badjson.MarshallObjects((_DNSRuleAction)(r))
	}
	return badjson.MarshallObjects((_DNSRuleAction)(r), v)
}

func (r *DNSRuleAction) UnmarshalJSONContext(ctx context.Context, data []byte) error {
	err := json.Unmarshal(data, (*_DNSRuleAction)(r))
	if err != nil {
		return err
	}
	var v any
	switch r.Action {
	case "", C.RuleActionTypeRoute:
		r.Action = C.RuleActionTypeRoute
		v = &r.RouteOptions
	case C.RuleActionTypeEvaluate:
		v = &r.EvaluateOptions
	case C.RuleActionTypeRespond:
		v = nil
	case C.RuleActionTypeRouteOptions:
		v = &r.RouteOptionsOptions
	case C.RuleActionTypeReject:
		v = &r.RejectOptions
	case C.RuleActionTypePredefined:
		v = &r.PredefinedOptions
	default:
		return E.New("unknown DNS rule action: " + r.Action)
	}
	if v == nil {
		return json.UnmarshalDisallowUnknownFields(data, &_DNSRuleAction{})
	}
	err = badjson.UnmarshallExcludedContext(ctx, data, (*_DNSRuleAction)(r), v)
	if err != nil {
		return err
	}
	return nil
}

type RouteActionOptions struct {
	Outbound string `json:"outbound,omitempty" reference:"outbound"`
	RawRouteOptionsActionOptions
}

type RawRouteOptionsActionOptions struct {
	OverrideAddress string `json:"override_address,omitempty"`
	OverridePort    uint16 `json:"override_port,omitempty"`

	NetworkStrategy *NetworkStrategy `json:"network_strategy,omitempty"`
	FallbackDelay   uint32           `json:"fallback_delay,omitempty"`

	UDPDisableDomainUnmapping bool               `json:"udp_disable_domain_unmapping,omitempty"`
	UDPConnect                bool               `json:"udp_connect,omitempty"`
	UDPTimeout                badoption.Duration `json:"udp_timeout,omitempty"`

	TLSFragment              bool               `json:"tls_fragment,omitempty"`
	TLSFragmentFallbackDelay badoption.Duration `json:"tls_fragment_fallback_delay,omitempty"`
	TLSRecordFragment        bool               `json:"tls_record_fragment,omitempty"`
	TLSSpoof                 string             `json:"tls_spoof,omitempty"`
	TLSSpoofMethod           string             `json:"tls_spoof_method,omitempty" enum:"wrong-sequence,wrong-checksum,wrong-ack,wrong-md5,wrong-timestamp"`
}

type RouteOptionsActionOptions RawRouteOptionsActionOptions

func (r *RouteOptionsActionOptions) UnmarshalJSON(data []byte) error {
	err := json.Unmarshal(data, (*RawRouteOptionsActionOptions)(r))
	if err != nil {
		return err
	}
	if *r == (RouteOptionsActionOptions{}) {
		return E.New("empty route option action")
	}
	if r.TLSFragment && r.TLSRecordFragment {
		return E.New("`tls_fragment` and `tls_record_fragment` are mutually exclusive")
	}
	return nil
}

type DNSRouteActionOptions struct {
	Server      string `json:"server,omitempty" reference:"dns_server"`
	Speculative bool   `json:"speculative,omitempty"`
	AbstractDNSRouteActionOptions
}

type DNSEvaluateActionOptions struct {
	Server      string `json:"server,omitempty" reference:"dns_server"`
	Tag         string `json:"tag,omitempty"`
	Speculative bool   `json:"speculative,omitempty"`
	AbstractDNSRouteActionOptions
}

type AbstractDNSRouteActionOptions struct {
	Timeout                badoption.Duration    `json:"timeout,omitempty"`
	Strategy               DomainStrategy        `json:"strategy,omitempty" schema:"omit"`
	DisableCache           bool                  `json:"disable_cache,omitempty"`
	DisableOptimisticCache bool                  `json:"disable_optimistic_cache,omitempty"`
	RewriteTTL             *uint32               `json:"rewrite_ttl,omitempty"`
	ClientSubnet           *badoption.Prefixable `json:"client_subnet,omitempty"`
}

type DNSRouteOptionsActionOptions AbstractDNSRouteActionOptions

func (r *DNSRouteOptionsActionOptions) UnmarshalJSON(data []byte) error {
	err := json.Unmarshal(data, (*AbstractDNSRouteActionOptions)(r))
	if err != nil {
		return err
	}
	if *r == (DNSRouteOptionsActionOptions{}) {
		return E.New("empty DNS route option action")
	}
	return nil
}

type DirectActionOptions struct {
	AbstractDialerOptions
}

func (d DirectActionOptions) Descriptions() []string {
	var descriptions []string
	if d.BindInterface != "" {
		descriptions = append(descriptions, "bind_interface="+d.BindInterface)
	}
	if d.Inet4BindAddress != nil {
		descriptions = append(descriptions, "inet4_bind_address="+d.Inet4BindAddress.Build(netip.IPv4Unspecified()).String())
	}
	if d.Inet6BindAddress != nil {
		descriptions = append(descriptions, "inet6_bind_address="+d.Inet6BindAddress.Build(netip.IPv6Unspecified()).String())
	}
	if d.RoutingMark != 0 {
		descriptions = append(descriptions, "routing_mark="+fmt.Sprintf("0x%x", d.RoutingMark))
	}
	if d.ReuseAddr {
		descriptions = append(descriptions, "reuse_addr")
	}
	if d.ConnectTimeout != 0 {
		descriptions = append(descriptions, "connect_timeout="+time.Duration(d.ConnectTimeout).String())
	}
	if d.TCPFastOpen {
		descriptions = append(descriptions, "tcp_fast_open")
	}
	if d.TCPMultiPath {
		descriptions = append(descriptions, "tcp_multi_path")
	}
	if d.UDPFragment != nil {
		descriptions = append(descriptions, "udp_fragment="+fmt.Sprint(*d.UDPFragment))
	}
	if d.DomainStrategy != DomainStrategy(C.DomainStrategyAsIS) {
		descriptions = append(descriptions, "domain_strategy="+d.DomainStrategy.String())
	}
	if d.FallbackDelay != 0 {
		descriptions = append(descriptions, "fallback_delay="+time.Duration(d.FallbackDelay).String())
	}
	return descriptions
}

type _RejectActionOptions struct {
	Method string `json:"method,omitempty" enum:"default,drop,reply"`
	NoDrop bool   `json:"no_drop,omitempty"`
}

type RejectActionOptions _RejectActionOptions

func (r RejectActionOptions) MarshalJSON() ([]byte, error) {
	switch r.Method {
	case C.RuleActionRejectMethodDefault:
		r.Method = ""
	}
	return json.Marshal((_RejectActionOptions)(r))
}

func (r *RejectActionOptions) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_RejectActionOptions)(r))
	if err != nil {
		return err
	}
	switch r.Method {
	case "", C.RuleActionRejectMethodDefault:
		r.Method = C.RuleActionRejectMethodDefault
	case C.RuleActionRejectMethodDrop:
	case C.RuleActionRejectMethodReply:
	default:
		return E.New("unknown reject method: " + r.Method)
	}
	if r.Method == C.RuleActionRejectMethodDrop && r.NoDrop {
		return E.New("no_drop is not available in current context")
	}
	return nil
}

type RouteActionSniff struct {
	Sniffer badoption.Listable[string] `json:"sniffer,omitempty" enum:"tls,http,quic,dns,stun,bittorrent,dtls,ssh,rdp,ntp"`
	Timeout badoption.Duration         `json:"timeout,omitempty"`
}

type RouteActionResolve struct {
	Server                 string                `json:"server,omitempty" reference:"dns_server"`
	Timeout                badoption.Duration    `json:"timeout,omitempty"`
	Strategy               DomainStrategy        `json:"strategy,omitempty"`
	DisableCache           bool                  `json:"disable_cache,omitempty"`
	DisableOptimisticCache bool                  `json:"disable_optimistic_cache,omitempty"`
	RewriteTTL             *uint32               `json:"rewrite_ttl,omitempty"`
	ClientSubnet           *badoption.Prefixable `json:"client_subnet,omitempty"`
}

type DNSRouteActionPredefined struct {
	Rcode  *DNSRCode                            `json:"rcode,omitempty"`
	Answer badoption.Listable[DNSRecordOptions] `json:"answer,omitempty"`
	Ns     badoption.Listable[DNSRecordOptions] `json:"ns,omitempty"`
	Extra  badoption.Listable[DNSRecordOptions] `json:"extra,omitempty"`
}

type actionVariant struct {
	action         string
	actionOptional bool
	structType     reflect.Type
	build          func(variant *schema.Node) error
}

func actionUnion(builder schema.Builder, variants []actionVariant) (*schema.Node, error) {
	variantNodes := make([]*schema.Node, 0, len(variants))
	for _, variant := range variants {
		variantNode := schema.LooseObject()
		variantNode.Properties.Put("action", schema.StringConst(variant.action))
		if !variant.actionOptional {
			variantNode.Required = []string{"action"}
		}
		if variant.build != nil {
			err := variant.build(variantNode)
			if err != nil {
				return nil, err
			}
		}
		if variant.structType != nil {
			err := builder.FlattenStruct(variantNode, variant.structType)
			if err != nil {
				return nil, err
			}
		}
		variantNodes = append(variantNodes, variantNode)
	}
	return schema.OneOf(variantNodes...), nil
}

func rejectProperties(variant *schema.Node) error {
	variant.Properties.Put("method", schema.StringEnum(
		"",
		C.RuleActionRejectMethodDefault,
		C.RuleActionRejectMethodDrop,
		C.RuleActionRejectMethodReply,
	))
	variant.Properties.Put("no_drop", schema.BooleanNode())
	return nil
}

func routeActionUnion(builder schema.Builder) (*schema.Node, error) {
	return actionUnion(builder, []actionVariant{
		{action: C.RuleActionTypeRoute, actionOptional: true, structType: reflect.TypeFor[RouteActionOptions]()},
		{action: C.RuleActionTypeRouteOptions, structType: reflect.TypeFor[RawRouteOptionsActionOptions]()},
		{action: C.RuleActionTypeDirect, structType: reflect.TypeFor[DirectActionOptions]()},
		{action: C.RuleActionTypeBypass, structType: reflect.TypeFor[RouteActionOptions]()},
		{action: C.RuleActionTypeReject, build: rejectProperties},
		{action: C.RuleActionTypeHijackDNS},
		{action: C.RuleActionTypeSniff, structType: reflect.TypeFor[RouteActionSniff]()},
		{action: C.RuleActionTypeResolve, structType: reflect.TypeFor[RouteActionResolve]()},
	})
}

func dnsActionUnion(builder schema.Builder) (*schema.Node, error) {
	raceProperty := func(variant *schema.Node) error {
		variant.Properties.Put("race", schema.BooleanNode())
		return nil
	}
	rejectWithRace := func(variant *schema.Node) error {
		err := raceProperty(variant)
		if err != nil {
			return err
		}
		return rejectProperties(variant)
	}
	return actionUnion(builder, []actionVariant{
		{action: C.RuleActionTypeRoute, actionOptional: true, structType: reflect.TypeFor[DNSRouteActionOptions](), build: raceProperty},
		{action: C.RuleActionTypeEvaluate, structType: reflect.TypeFor[DNSEvaluateActionOptions](), build: raceProperty},
		{action: C.RuleActionTypeRespond, build: raceProperty},
		{action: C.RuleActionTypeRouteOptions, structType: reflect.TypeFor[DNSRouteOptionsActionOptions](), build: raceProperty},
		{action: C.RuleActionTypeReject, build: rejectWithRace},
		{action: C.RuleActionTypePredefined, structType: reflect.TypeFor[DNSRouteActionPredefined](), build: raceProperty},
	})
}
