package option

import (
	"context"
	"net/netip"

	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/service"
)

type RawDNSOptions struct {
	Servers        []DNSServerOptions `json:"servers,omitempty"`
	Rules          []DNSRule          `json:"rules,omitempty"`
	Final          string             `json:"final,omitempty"`
	ReverseMapping bool               `json:"reverse_mapping,omitempty"`
	DNSClientOptions
}

type DNSOptions struct {
	RawDNSOptions
}

const (
	legacyDNSFakeIPRemovedMessage = "legacy DNS fakeip options are deprecated in sing-box 1.12.0 and removed in sing-box 1.14.0, checkout migration: https://sing-box.sagernet.org/migration/#migrate-to-new-dns-server-formats"
	legacyDNSServerRemovedMessage = "legacy DNS server formats are deprecated in sing-box 1.12.0 and removed in sing-box 1.14.0, checkout migration: https://sing-box.sagernet.org/migration/#migrate-to-new-dns-server-formats"
)

type removedLegacyDNSOptions struct {
	FakeIP json.RawMessage `json:"fakeip,omitempty"`
}

func (o *DNSOptions) UnmarshalJSONContext(ctx context.Context, content []byte) error {
	var legacyOptions removedLegacyDNSOptions
	err := json.UnmarshalContext(ctx, content, &legacyOptions)
	if err != nil {
		return err
	}
	if len(legacyOptions.FakeIP) != 0 {
		return E.New(legacyDNSFakeIPRemovedMessage)
	}
	return badjson.UnmarshallExcludedContext(ctx, content, legacyOptions, &o.RawDNSOptions)
}

type DNSClientOptions struct {
	Strategy         DomainStrategy        `json:"strategy,omitempty"`
	DisableCache     bool                  `json:"disable_cache,omitempty"`
	DisableExpire    bool                  `json:"disable_expire,omitempty"`
	IndependentCache bool                  `json:"independent_cache,omitempty"`
	CacheCapacity    uint32                `json:"cache_capacity,omitempty"`
	ClientSubnet     *badoption.Prefixable `json:"client_subnet,omitempty"`
}

type DNSTransportOptionsRegistry interface {
	CreateOptions(transportType string) (any, bool)
}
type _DNSServerOptions struct {
	Type    string `json:"type,omitempty"`
	Tag     string `json:"tag,omitempty"`
	Options any    `json:"-"`
}

type DNSServerOptions _DNSServerOptions

func (o *DNSServerOptions) MarshalJSONContext(ctx context.Context) ([]byte, error) {
	return badjson.MarshallObjectsContext(ctx, (*_DNSServerOptions)(o), o.Options)
}

func (o *DNSServerOptions) UnmarshalJSONContext(ctx context.Context, content []byte) error {
	err := json.UnmarshalContext(ctx, content, (*_DNSServerOptions)(o))
	if err != nil {
		return err
	}
	registry := service.FromContext[DNSTransportOptionsRegistry](ctx)
	if registry == nil {
		return E.New("missing DNS transport options registry in context")
	}
	var options any
	switch o.Type {
	case "", C.DNSTypeLegacy:
		return E.New(legacyDNSServerRemovedMessage)
	default:
		var loaded bool
		options, loaded = registry.CreateOptions(o.Type)
		if !loaded {
			return E.New("unknown transport type: ", o.Type)
		}
	}
	err = badjson.UnmarshallExcludedContext(ctx, content, (*_DNSServerOptions)(o), options)
	if err != nil {
		return err
	}
	o.Options = options
	return nil
}

type DNSServerAddressOptions struct {
	Server     string `json:"server"`
	ServerPort uint16 `json:"server_port,omitempty"`
}

func (o DNSServerAddressOptions) Build() M.Socksaddr {
	return M.ParseSocksaddrHostPort(o.Server, o.ServerPort)
}

func (o DNSServerAddressOptions) ServerIsDomain() bool {
	return o.Build().IsDomain()
}

func (o *DNSServerAddressOptions) TakeServerOptions() ServerOptions {
	return ServerOptions(*o)
}

func (o *DNSServerAddressOptions) ReplaceServerOptions(options ServerOptions) {
	*o = DNSServerAddressOptions(options)
}

type HostsDNSServerOptions struct {
	Path       badoption.Listable[string]                                `json:"path,omitempty"`
	Predefined *badjson.TypedMap[string, badoption.Listable[netip.Addr]] `json:"predefined,omitempty"`
}

type RawLocalDNSServerOptions struct {
	DialerOptions
}

type LocalDNSServerOptions struct {
	RawLocalDNSServerOptions
	PreferGo bool `json:"prefer_go,omitempty"`
}

type RemoteDNSServerOptions struct {
	RawLocalDNSServerOptions
	DNSServerAddressOptions
}

type RemoteTLSDNSServerOptions struct {
	RemoteDNSServerOptions
	OutboundTLSOptionsContainer
}

type RemoteHTTPSDNSServerOptions struct {
	RemoteTLSDNSServerOptions
	Path    string               `json:"path,omitempty"`
	Method  string               `json:"method,omitempty"`
	Headers badoption.HTTPHeader `json:"headers,omitempty"`
}

type FakeIPDNSServerOptions struct {
	Inet4Range *badoption.Prefix `json:"inet4_range,omitempty"`
	Inet6Range *badoption.Prefix `json:"inet6_range,omitempty"`
}

type DHCPDNSServerOptions struct {
	LocalDNSServerOptions
	Interface string `json:"interface,omitempty"`
}
