package option

import (
	"context"
	"reflect"

	"github.com/sagernet/sing-box/schema"
	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
)

type SocksInboundOptions struct {
	ListenOptions
	Users          []auth.User           `json:"users,omitempty"`
	DomainResolver *DomainResolveOptions `json:"domain_resolver,omitempty"`
}

type HTTPMixedInboundOptions struct {
	ListenOptions
	Users          []auth.User           `json:"users,omitempty"`
	DomainResolver *DomainResolveOptions `json:"domain_resolver,omitempty"`
	SetSystemProxy bool                  `json:"set_system_proxy,omitempty"`
	InboundTLSOptionsContainer
}

type _HTTPInboundOptions struct {
	ListenOptions
	Users          []auth.User             `json:"users,omitempty"`
	DomainResolver *DomainResolveOptions   `json:"domain_resolver,omitempty"`
	SetSystemProxy bool                    `json:"set_system_proxy,omitempty"`
	Version        badoption.Listable[int] `json:"version,omitempty" enum:"1,2,3"`
	InboundTLSOptionsContainer
	HTTP2Options HTTP2Options `json:"-"`
	HTTP3Options QUICOptions  `json:"-"`
}

type HTTPInboundOptions _HTTPInboundOptions

func (o HTTPInboundOptions) Versions() []int {
	if len(o.Version) > 0 {
		return o.Version
	}
	return []int{1, 2}
}

func (o HTTPInboundOptions) MarshalJSON() ([]byte, error) {
	return badjson.MarshallObjects(_HTTPInboundOptions(o), httpVersionsVariant(o.Versions(), o.HTTP2Options, o.HTTP3Options))
}

func (o *HTTPInboundOptions) UnmarshalJSONContext(ctx context.Context, content []byte) error {
	err := json.UnmarshalContext(ctx, content, (*_HTTPInboundOptions)(o))
	if err != nil {
		return err
	}
	return unmarshalHTTPVersionsOptions(ctx, content, (*_HTTPInboundOptions)(o), o.Versions(), &o.HTTP2Options, &o.HTTP3Options)
}

func (o HTTPInboundOptions) DescribeSchema(builder schema.Builder) (*schema.Node, error) {
	node := schema.StrictObject()
	err := builder.FlattenStruct(node, reflect.TypeFor[HTTPInboundOptions]())
	if err != nil {
		return nil, err
	}
	err = builder.FlattenStruct(node, reflect.TypeFor[QUICOptions]())
	if err != nil {
		return nil, err
	}
	return node, nil
}

type SOCKSOutboundOptions struct {
	DialerOptions
	ServerOptions
	Version    string             `json:"version,omitempty" enum:"4,4a,5"`
	Username   string             `json:"username,omitempty"`
	Password   string             `json:"password,omitempty"`
	Network    NetworkList        `json:"network,omitempty"`
	UDPOverTCP *UDPOverTCPOptions `json:"udp_over_tcp,omitempty"`
}

type _HTTPOutboundOptions struct {
	DialerOptions
	ServerOptions
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	OutboundTLSOptionsContainer
	Path                   string               `json:"path,omitempty"`
	Headers                badoption.HTTPHeader `json:"headers,omitempty"`
	Version                int                  `json:"version,omitempty" enum:"0,1,2,3"`
	DisableVersionFallback bool                 `json:"disable_version_fallback,omitempty"`
	HTTP2Options           HTTP2Options         `json:"-"`
	HTTP3Options           QUICOptions          `json:"-"`
}

type HTTPOutboundOptions _HTTPOutboundOptions

func (o HTTPOutboundOptions) MarshalJSON() ([]byte, error) {
	return badjson.MarshallObjects(_HTTPOutboundOptions(o), httpVersionVariant(o.Version, o.HTTP2Options, o.HTTP3Options))
}

func (o *HTTPOutboundOptions) UnmarshalJSONContext(ctx context.Context, content []byte) error {
	err := json.UnmarshalContext(ctx, content, (*_HTTPOutboundOptions)(o))
	if err != nil {
		return err
	}
	return unmarshalHTTPVersionOptions(ctx, content, (*_HTTPOutboundOptions)(o), o.Version, &o.HTTP2Options, &o.HTTP3Options)
}

func (o HTTPOutboundOptions) DescribeSchema(builder schema.Builder) (*schema.Node, error) {
	node := schema.StrictObject()
	err := builder.FlattenStruct(node, reflect.TypeFor[HTTPOutboundOptions]())
	if err != nil {
		return nil, err
	}
	err = builder.FlattenStruct(node, reflect.TypeFor[QUICOptions]())
	if err != nil {
		return nil, err
	}
	return node, nil
}
