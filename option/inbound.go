package option

import (
	"context"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/schema"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
	"github.com/sagernet/sing/service"
)

type InboundOptionsRegistry interface {
	OptionTypes() []string
	CreateOptions(outboundType string) (any, bool)
}

type _Inbound struct {
	Type    string `json:"type"`
	Tag     string `json:"tag,omitempty"`
	Options any    `json:"-"`
}

type Inbound _Inbound

func (h *Inbound) MarshalJSONContext(ctx context.Context) ([]byte, error) {
	return badjson.MarshallObjectsContext(ctx, (*_Inbound)(h), h.Options)
}

func (h *Inbound) UnmarshalJSONContext(ctx context.Context, content []byte) error {
	err := json.UnmarshalContext(ctx, content, (*_Inbound)(h))
	if err != nil {
		return err
	}
	registry := service.FromContext[InboundOptionsRegistry](ctx)
	if registry == nil {
		return E.New("missing inbound fields registry in context")
	}
	options, loaded := registry.CreateOptions(h.Type)
	if !loaded {
		return E.New("unknown inbound type: ", h.Type)
	}
	err = badjson.UnmarshallExcludedContext(ctx, content, (*_Inbound)(h), options)
	if err != nil {
		return err
	}
	if listenWrapper, isListen := options.(ListenOptionsWrapper); isListen {
		//nolint:staticcheck
		if listenWrapper.TakeListenOptions().InboundOptions != (InboundOptions{}) {
			return E.New("legacy inbound fields are deprecated in sing-box 1.11.0 and removed in sing-box 1.13.0, checkout migration: https://sing-box.sagernet.org/migration/#migrate-legacy-inbound-fields-to-rule-actions")
		}
	}
	h.Options = options
	return nil
}

func (h Inbound) DescribeSchema(builder schema.Builder) (*schema.Node, error) {
	return builder.Define("Inbound", func() (*schema.Node, error) {
		registry := service.FromContext[InboundOptionsRegistry](builder.Context())
		if registry == nil {
			return nil, E.New("missing inbound options registry in context")
		}
		return registryUnion(builder, registry, []string{C.TypeShadowsocksR}, true)
	})
}

// Deprecated: Use rule action instead
type InboundOptions struct {
	SniffEnabled              bool               `json:"sniff,omitempty" schema:"omit"`
	SniffOverrideDestination  bool               `json:"sniff_override_destination,omitempty" schema:"omit"`
	SniffTimeout              badoption.Duration `json:"sniff_timeout,omitempty" schema:"omit"`
	DomainStrategy            DomainStrategy     `json:"domain_strategy,omitempty" schema:"omit"`
	UDPDisableDomainUnmapping bool               `json:"udp_disable_domain_unmapping,omitempty" schema:"omit"`
}

type ListenOptions struct {
	Listen               *badoption.Addr    `json:"listen,omitempty"`
	ListenPort           uint16             `json:"listen_port,omitempty"`
	BindInterface        string             `json:"bind_interface,omitempty"`
	RoutingMark          FwMark             `json:"routing_mark,omitempty"`
	ReuseAddr            bool               `json:"reuse_addr,omitempty"`
	NetNs                string             `json:"netns,omitempty" reference:"network_namespace"`
	DisableTCPKeepAlive  bool               `json:"disable_tcp_keep_alive,omitempty"`
	TCPKeepAlive         badoption.Duration `json:"tcp_keep_alive,omitempty"`
	TCPKeepAliveInterval badoption.Duration `json:"tcp_keep_alive_interval,omitempty"`
	TCPFastOpen          bool               `json:"tcp_fast_open,omitempty"`
	TCPMultiPath         bool               `json:"tcp_multi_path,omitempty"`
	UDPFragment          *bool              `json:"udp_fragment,omitempty"`
	UDPFragmentDefault   bool               `json:"-"`
	UDPTimeout           UDPTimeoutCompat   `json:"udp_timeout,omitempty"`
	Detour               string             `json:"detour,omitempty" reference:"inbound"`

	// Deprecated: removed
	ProxyProtocol bool `json:"proxy_protocol,omitempty" schema:"omit"`
	// Deprecated: removed
	ProxyProtocolAcceptNoHeader bool `json:"proxy_protocol_accept_no_header,omitempty" schema:"omit"`
	// Legacy inbound fields are rejected since sing-box 1.13.0.
	//nolint:staticcheck
	InboundOptions `schema:"omit"`
}

type UDPNATBehavior uint8

const (
	UDPNATBehaviorEndpointIndependent UDPNATBehavior = iota
	UDPNATBehaviorAddressDependent
	UDPNATBehaviorAddressAndPortDependent
)

func (b UDPNATBehavior) MarshalJSON() ([]byte, error) {
	var value string
	switch b {
	case UDPNATBehaviorEndpointIndependent:
		value = "endpoint_independent"
	case UDPNATBehaviorAddressDependent:
		value = "address_dependent"
	case UDPNATBehaviorAddressAndPortDependent:
		value = "address_and_port_dependent"
	default:
		return nil, E.New("unknown UDP NAT behavior: ", uint8(b))
	}
	return json.Marshal(value)
}

func (b *UDPNATBehavior) UnmarshalJSON(data []byte) error {
	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return err
	}
	switch value {
	case "", "endpoint_independent":
		*b = UDPNATBehaviorEndpointIndependent
	case "address_dependent":
		*b = UDPNATBehaviorAddressDependent
	case "address_and_port_dependent":
		*b = UDPNATBehaviorAddressAndPortDependent
	default:
		return E.New("unknown UDP NAT behavior: ", value)
	}
	return nil
}

func (b UDPNATBehavior) DescribeSchema(builder schema.Builder) (*schema.Node, error) {
	return schema.StringEnum("", "endpoint_independent", "address_dependent", "address_and_port_dependent"), nil
}

type UDPTimeoutCompat badoption.Duration

func (c UDPTimeoutCompat) MarshalJSON() ([]byte, error) {
	return json.Marshal((time.Duration)(c).String())
}

func (c *UDPTimeoutCompat) UnmarshalJSON(data []byte) error {
	var valueNumber int64
	err := json.Unmarshal(data, &valueNumber)
	if err == nil {
		*c = UDPTimeoutCompat(time.Second * time.Duration(valueNumber))
		return nil
	}
	return json.Unmarshal(data, (*badoption.Duration)(c))
}

func (c UDPTimeoutCompat) DescribeSchema(builder schema.Builder) (*schema.Node, error) {
	return builder.Define("UDPTimeout", func() (*schema.Node, error) {
		return schema.AnyOf(schema.UnsignedNode(32), schema.DurationNode()), nil
	})
}

type ListenOptionsWrapper interface {
	TakeListenOptions() ListenOptions
	ReplaceListenOptions(options ListenOptions)
}

func (o *ListenOptions) TakeListenOptions() ListenOptions {
	return *o
}

func (o *ListenOptions) ReplaceListenOptions(options ListenOptions) {
	*o = options
}
