package option

import (
	"context"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/deprecated"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/service"
)

type OutboundOptionsRegistry interface {
	CreateOptions(outboundType string) (any, bool)
}

type _Outbound struct {
	Type    string `json:"type"`
	Tag     string `json:"tag,omitempty"`
	Options any    `json:"-"`
}

type Outbound _Outbound

func (h *Outbound) MarshalJSONContext(ctx context.Context) ([]byte, error) {
	return badjson.MarshallObjectsContext(ctx, (*_Outbound)(h), h.Options)
}

func (h *Outbound) UnmarshalJSONContext(ctx context.Context, content []byte) error {
	err := json.UnmarshalContext(ctx, content, (*_Outbound)(h))
	if err != nil {
		return err
	}
	registry := service.FromContext[OutboundOptionsRegistry](ctx)
	if registry == nil {
		return E.New("missing outbound options registry in context")
	}
	switch h.Type {
	case C.TypeDNS:
		deprecated.Report(ctx, deprecated.OptionSpecialOutbounds)
	}
	options, loaded := registry.CreateOptions(h.Type)
	if !loaded {
		return E.New("unknown outbound type: ", h.Type)
	}
	err = badjson.UnmarshallExcludedContext(ctx, content, (*_Outbound)(h), options)
	if err != nil {
		return err
	}
	if listenWrapper, isListen := options.(ListenOptionsWrapper); isListen {
		if listenWrapper.TakeListenOptions().InboundOptions != (InboundOptions{}) {
			deprecated.Report(ctx, deprecated.OptionInboundOptions)
		}
	}
	h.Options = options
	return nil
}

type DialerOptionsWrapper interface {
	TakeDialerOptions() DialerOptions
	ReplaceDialerOptions(options DialerOptions)
}

type DialerOptions struct {
	Detour              string                            `json:"detour,omitempty"`
	BindInterface       string                            `json:"bind_interface,omitempty"`
	Inet4BindAddress    *badoption.Addr                   `json:"inet4_bind_address,omitempty"`
	Inet6BindAddress    *badoption.Addr                   `json:"inet6_bind_address,omitempty"`
	ProtectPath         string                            `json:"protect_path,omitempty"`
	RoutingMark         FwMark                            `json:"routing_mark,omitempty"`
	ReuseAddr           bool                              `json:"reuse_addr,omitempty"`
	NetNs               string                            `json:"netns,omitempty"`
	ConnectTimeout      badoption.Duration                `json:"connect_timeout,omitempty"`
	TCPFastOpen         bool                              `json:"tcp_fast_open,omitempty"`
	TCPMultiPath        bool                              `json:"tcp_multi_path,omitempty"`
	UDPFragment         *bool                             `json:"udp_fragment,omitempty"`
	UDPFragmentDefault  bool                              `json:"-"`
	DomainResolver      *DomainResolveOptions             `json:"domain_resolver,omitempty"`
	NetworkStrategy     *NetworkStrategy                  `json:"network_strategy,omitempty"`
	NetworkType         badoption.Listable[InterfaceType] `json:"network_type,omitempty"`
	FallbackNetworkType badoption.Listable[InterfaceType] `json:"fallback_network_type,omitempty"`
	FallbackDelay       badoption.Duration                `json:"fallback_delay,omitempty"`

	// Deprecated: migrated to domain resolver
	DomainStrategy DomainStrategy `json:"domain_strategy,omitempty"`
}

type _DomainResolveOptions struct {
	Server       string                `json:"server"`
	Strategy     DomainStrategy        `json:"strategy,omitempty"`
	DisableCache bool                  `json:"disable_cache,omitempty"`
	RewriteTTL   *uint32               `json:"rewrite_ttl,omitempty"`
	ClientSubnet *badoption.Prefixable `json:"client_subnet,omitempty"`
}

type DomainResolveOptions _DomainResolveOptions

func (o DomainResolveOptions) MarshalJSON() ([]byte, error) {
	if o.Server == "" {
		return []byte("{}"), nil
	} else if o.Strategy == DomainStrategy(C.DomainStrategyAsIS) &&
		!o.DisableCache &&
		o.RewriteTTL == nil &&
		o.ClientSubnet == nil {
		return json.Marshal(o.Server)
	} else {
		return json.Marshal((_DomainResolveOptions)(o))
	}
}

func (o *DomainResolveOptions) UnmarshalJSON(bytes []byte) error {
	var stringValue string
	err := json.Unmarshal(bytes, &stringValue)
	if err == nil {
		o.Server = stringValue
		return nil
	}
	err = json.Unmarshal(bytes, (*_DomainResolveOptions)(o))
	if err != nil {
		return err
	}
	if o.Server == "" {
		return E.New("empty domain_resolver.server")
	}
	return nil
}

func (o *DialerOptions) TakeDialerOptions() DialerOptions {
	return *o
}

func (o *DialerOptions) ReplaceDialerOptions(options DialerOptions) {
	*o = options
}

type ServerOptionsWrapper interface {
	TakeServerOptions() ServerOptions
	ReplaceServerOptions(options ServerOptions)
}

type ServerOptions struct {
	Server     string `json:"server"`
	ServerPort uint16 `json:"server_port"`
}

func (o ServerOptions) Build() M.Socksaddr {
	return M.ParseSocksaddrHostPort(o.Server, o.ServerPort)
}

func (o ServerOptions) ServerIsDomain() bool {
	return M.IsDomainName(o.Server)
}

func (o *ServerOptions) TakeServerOptions() ServerOptions {
	return *o
}

func (o *ServerOptions) ReplaceServerOptions(options ServerOptions) {
	*o = options
}
