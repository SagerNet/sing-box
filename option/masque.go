package option

import (
	"context"
	"net/netip"
	"reflect"

	"github.com/sagernet/sing-box/schema"
	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
)

type MASQUEEndpointOptions struct {
	System       bool           `json:"system,omitempty"`
	Name         string         `json:"name,omitempty"`
	MTU          uint32         `json:"mtu,omitempty"`
	UDPMapping   UDPNATBehavior `json:"udp_mapping,omitempty"`
	UDPFiltering UDPNATBehavior `json:"udp_filtering,omitempty"`
	UDPNATMax    uint32         `json:"udp_nat_max,omitempty"`
}

type _MASQUEClientEndpointOptions struct {
	DialerOptions
	ServerOptions
	MASQUEEndpointOptions
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	OutboundTLSOptionsContainer
	Path                   string                           `json:"path,omitempty"`
	Headers                badoption.HTTPHeader             `json:"headers,omitempty"`
	Version                int                              `json:"version,omitempty" enum:"0,1,2,3"`
	DisableVersionFallback bool                             `json:"disable_version_fallback,omitempty"`
	AdvertiseRoutes        badoption.Listable[netip.Prefix] `json:"advertise_routes,omitempty"`
	UDPTimeout             badoption.Duration               `json:"udp_timeout,omitempty"`
	OnDemand               bool                             `json:"on_demand,omitempty"`
	HTTP2Options           HTTP2Options                     `json:"-"`
	HTTP3Options           QUICOptions                      `json:"-"`
}

type MASQUEClientEndpointOptions _MASQUEClientEndpointOptions

func (o MASQUEClientEndpointOptions) ResolvedVersion() int {
	if o.Version == 0 {
		return 3
	}
	return o.Version
}

func (o MASQUEClientEndpointOptions) MarshalJSON() ([]byte, error) {
	return badjson.MarshallObjects(_MASQUEClientEndpointOptions(o), httpVersionVariant(o.ResolvedVersion(), o.HTTP2Options, o.HTTP3Options))
}

func (o *MASQUEClientEndpointOptions) UnmarshalJSONContext(ctx context.Context, content []byte) error {
	err := json.UnmarshalContext(ctx, content, (*_MASQUEClientEndpointOptions)(o))
	if err != nil {
		return err
	}
	return unmarshalHTTPVersionOptions(ctx, content, (*_MASQUEClientEndpointOptions)(o), o.ResolvedVersion(), &o.HTTP2Options, &o.HTTP3Options)
}

func (o MASQUEClientEndpointOptions) DescribeSchema(builder schema.Builder) (*schema.Node, error) {
	node := schema.StrictObject()
	err := builder.FlattenStruct(node, reflect.TypeFor[MASQUEClientEndpointOptions]())
	if err != nil {
		return nil, err
	}
	err = builder.FlattenStruct(node, reflect.TypeFor[QUICOptions]())
	if err != nil {
		return nil, err
	}
	return node, nil
}

type _MASQUEServerEndpointOptions struct {
	ListenOptions
	MASQUEEndpointOptions
	Users   []auth.User             `json:"users,omitempty"`
	Version badoption.Listable[int] `json:"version,omitempty" enum:"1,2,3"`
	InboundTLSOptionsContainer
	Path            string                           `json:"path,omitempty"`
	Address         badoption.Listable[netip.Prefix] `json:"address"`
	AdvertiseRoutes badoption.Listable[netip.Prefix] `json:"advertise_routes,omitempty"`
	HTTP2Options    HTTP2Options                     `json:"-"`
	HTTP3Options    QUICOptions                      `json:"-"`
}

type MASQUEServerEndpointOptions _MASQUEServerEndpointOptions

func (o MASQUEServerEndpointOptions) Versions() []int {
	if len(o.Version) > 0 {
		return o.Version
	}
	return []int{1, 2, 3}
}

func (o MASQUEServerEndpointOptions) MarshalJSON() ([]byte, error) {
	return badjson.MarshallObjects(_MASQUEServerEndpointOptions(o), httpVersionsVariant(o.Versions(), o.HTTP2Options, o.HTTP3Options))
}

func (o *MASQUEServerEndpointOptions) UnmarshalJSONContext(ctx context.Context, content []byte) error {
	err := json.UnmarshalContext(ctx, content, (*_MASQUEServerEndpointOptions)(o))
	if err != nil {
		return err
	}
	return unmarshalHTTPVersionsOptions(ctx, content, (*_MASQUEServerEndpointOptions)(o), o.Versions(), &o.HTTP2Options, &o.HTTP3Options)
}

func (o MASQUEServerEndpointOptions) DescribeSchema(builder schema.Builder) (*schema.Node, error) {
	node := schema.StrictObject()
	err := builder.FlattenStruct(node, reflect.TypeFor[MASQUEServerEndpointOptions]())
	if err != nil {
		return nil, err
	}
	err = builder.FlattenStruct(node, reflect.TypeFor[QUICOptions]())
	if err != nil {
		return nil, err
	}
	return node, nil
}
