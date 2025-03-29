package option

import (
	"context"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
	"github.com/sagernet/sing/service"
)

type InboundOptionsRegistry interface {
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
	h.Options = options
	return nil
}

// Deprecated: Use rule action instead
type InboundOptions struct {
	SniffEnabled              bool               `json:"sniff,omitempty"`
	SniffOverrideDestination  bool               `json:"sniff_override_destination,omitempty"`
	SniffTimeout              badoption.Duration `json:"sniff_timeout,omitempty"`
	DomainStrategy            DomainStrategy     `json:"domain_strategy,omitempty"`
	UDPDisableDomainUnmapping bool               `json:"udp_disable_domain_unmapping,omitempty"`
	Detour                    string             `json:"detour,omitempty"`
}

type ListenOptions struct {
	Listen               *badoption.Addr    `json:"listen,omitempty"`
	ListenPort           uint16             `json:"listen_port,omitempty"`
	BindInterface        string             `json:"bind_interface,omitempty"`
	RoutingMark          FwMark             `json:"routing_mark,omitempty"`
	ReuseAddr            bool               `json:"reuse_addr,omitempty"`
	NetNs                string             `json:"netns,omitempty"`
	TCPKeepAlive         badoption.Duration `json:"tcp_keep_alive,omitempty"`
	TCPKeepAliveInterval badoption.Duration `json:"tcp_keep_alive_interval,omitempty"`
	TCPFastOpen          bool               `json:"tcp_fast_open,omitempty"`
	TCPMultiPath         bool               `json:"tcp_multi_path,omitempty"`
	UDPFragment          *bool              `json:"udp_fragment,omitempty"`
	UDPFragmentDefault   bool               `json:"-"`
	UDPTimeout           UDPTimeoutCompat   `json:"udp_timeout,omitempty"`

	// Deprecated: removed
	ProxyProtocol bool `json:"proxy_protocol,omitempty"`
	// Deprecated: removed
	ProxyProtocolAcceptNoHeader bool `json:"proxy_protocol_accept_no_header,omitempty"`
	InboundOptions
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
