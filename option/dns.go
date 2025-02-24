package option

import (
	"context"
	"net/netip"
	"net/url"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/deprecated"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/service"

	"github.com/miekg/dns"
)

type RawDNSOptions struct {
	Servers        []NewDNSServerOptions `json:"servers,omitempty"`
	Rules          []DNSRule             `json:"rules,omitempty"`
	Final          string                `json:"final,omitempty"`
	ReverseMapping bool                  `json:"reverse_mapping,omitempty"`
	DNSClientOptions
}

type LegacyDNSOptions struct {
	FakeIP *LegacyDNSFakeIPOptions `json:"fakeip,omitempty"`
}

type DNSOptions struct {
	RawDNSOptions
	LegacyDNSOptions
}

func (o *DNSOptions) UnmarshalJSONContext(ctx context.Context, content []byte) error {
	err := json.UnmarshalContext(ctx, content, &o.LegacyDNSOptions)
	if err != nil {
		return err
	}
	if o.FakeIP != nil && o.FakeIP.Enabled {
		deprecated.Report(ctx, deprecated.OptionLegacyDNSFakeIPOptions)
		ctx = context.WithValue(ctx, (*LegacyDNSFakeIPOptions)(nil), o.FakeIP)
	}
	legacyOptions := o.LegacyDNSOptions
	o.LegacyDNSOptions = LegacyDNSOptions{}
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

type LegacyDNSFakeIPOptions struct {
	Enabled    bool              `json:"enabled,omitempty"`
	Inet4Range *badoption.Prefix `json:"inet4_range,omitempty"`
	Inet6Range *badoption.Prefix `json:"inet6_range,omitempty"`
}

type DNSTransportOptionsRegistry interface {
	CreateOptions(transportType string) (any, bool)
}

type _NewDNSServerOptions struct {
	Type    string `json:"type,omitempty"`
	Tag     string `json:"tag,omitempty"`
	Options any    `json:"-"`
}

type NewDNSServerOptions _NewDNSServerOptions

func (o *NewDNSServerOptions) MarshalJSONContext(ctx context.Context) ([]byte, error) {
	return badjson.MarshallObjectsContext(ctx, (*_NewDNSServerOptions)(o), o.Options)
}

func (o *NewDNSServerOptions) UnmarshalJSONContext(ctx context.Context, content []byte) error {
	err := json.UnmarshalContext(ctx, content, (*_NewDNSServerOptions)(o))
	if err != nil {
		return err
	}
	registry := service.FromContext[DNSTransportOptionsRegistry](ctx)
	if registry == nil {
		return E.New("missing outbound options registry in context")
	}
	var options any
	switch o.Type {
	case "", C.DNSTypeLegacy:
		o.Type = C.DNSTypeLegacy
		options = new(LegacyDNSServerOptions)
		deprecated.Report(ctx, deprecated.OptionLegacyDNSTransport)
	default:
		var loaded bool
		options, loaded = registry.CreateOptions(o.Type)
		if !loaded {
			return E.New("unknown transport type: ", o.Type)
		}
	}
	err = badjson.UnmarshallExcludedContext(ctx, content, (*_Outbound)(o), options)
	if err != nil {
		return err
	}
	o.Options = options
	if o.Type == C.DNSTypeLegacy {
		err = o.Upgrade(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *NewDNSServerOptions) Upgrade(ctx context.Context) error {
	if o.Type != C.DNSTypeLegacy {
		return nil
	}
	options := o.Options.(*LegacyDNSServerOptions)
	serverURL, _ := url.Parse(options.Address)
	var serverType string
	if serverURL.Scheme != "" {
		serverType = serverURL.Scheme
	} else {
		switch options.Address {
		case "local", "fakeip":
			serverType = options.Address
		default:
			serverType = C.DNSTypeUDP
		}
	}
	var remoteOptions RemoteDNSServerOptions
	if options.Detour == "" {
		remoteOptions = RemoteDNSServerOptions{
			LocalDNSServerOptions: LocalDNSServerOptions{
				LegacyStrategy:      options.Strategy,
				LegacyDefaultDialer: options.Detour == "",
				LegacyClientSubnet:  options.ClientSubnet.Build(netip.Prefix{}),
			},
			LegacyAddressResolver:      options.AddressResolver,
			LegacyAddressStrategy:      options.AddressStrategy,
			LegacyAddressFallbackDelay: options.AddressFallbackDelay,
		}
	} else {
		remoteOptions = RemoteDNSServerOptions{
			LocalDNSServerOptions: LocalDNSServerOptions{
				DialerOptions: DialerOptions{
					Detour: options.Detour,
					DomainResolver: &DomainResolveOptions{
						Server:   options.AddressResolver,
						Strategy: options.AddressStrategy,
					},
					FallbackDelay: options.AddressFallbackDelay,
				},
				LegacyStrategy:      options.Strategy,
				LegacyDefaultDialer: options.Detour == "",
				LegacyClientSubnet:  options.ClientSubnet.Build(netip.Prefix{}),
			},
		}
	}
	switch serverType {
	case C.DNSTypeLocal:
		o.Type = C.DNSTypeLocal
		o.Options = &remoteOptions.LocalDNSServerOptions
	case C.DNSTypeUDP:
		o.Type = C.DNSTypeUDP
		o.Options = &remoteOptions
		var serverAddr M.Socksaddr
		if serverURL.Scheme == "" {
			serverAddr = M.ParseSocksaddr(options.Address)
		} else {
			serverAddr = M.ParseSocksaddr(serverURL.Host)
		}
		if !serverAddr.IsValid() {
			return E.New("invalid server address")
		}
		remoteOptions.Server = serverAddr.Addr.String()
		if serverAddr.Port != 0 && serverAddr.Port != 53 {
			remoteOptions.ServerPort = serverAddr.Port
		}
		remoteOptions.Server = serverAddr.AddrString()
		remoteOptions.ServerPort = serverAddr.Port
	case C.DNSTypeTCP:
		o.Type = C.DNSTypeTCP
		o.Options = &remoteOptions
		serverAddr := M.ParseSocksaddr(serverURL.Host)
		if !serverAddr.IsValid() {
			return E.New("invalid server address")
		}
		remoteOptions.Server = serverAddr.Addr.String()
		if serverAddr.Port != 0 && serverAddr.Port != 53 {
			remoteOptions.ServerPort = serverAddr.Port
		}
		remoteOptions.Server = serverAddr.AddrString()
		remoteOptions.ServerPort = serverAddr.Port
	case C.DNSTypeTLS, C.DNSTypeQUIC:
		o.Type = serverType
		serverAddr := M.ParseSocksaddr(serverURL.Host)
		if !serverAddr.IsValid() {
			return E.New("invalid server address")
		}
		remoteOptions.Server = serverAddr.Addr.String()
		if serverAddr.Port != 0 && serverAddr.Port != 853 {
			remoteOptions.ServerPort = serverAddr.Port
		}
		o.Options = &RemoteTLSDNSServerOptions{
			RemoteDNSServerOptions: remoteOptions,
		}
	case C.DNSTypeHTTPS, C.DNSTypeHTTP3:
		o.Type = serverType
		httpsOptions := RemoteHTTPSDNSServerOptions{
			RemoteTLSDNSServerOptions: RemoteTLSDNSServerOptions{
				RemoteDNSServerOptions: remoteOptions,
			},
		}
		o.Options = &httpsOptions
		serverAddr := M.ParseSocksaddr(serverURL.Host)
		if !serverAddr.IsValid() {
			return E.New("invalid server address")
		}
		httpsOptions.Server = serverAddr.Addr.String()
		if serverAddr.Port != 0 && serverAddr.Port != 443 {
			httpsOptions.ServerPort = serverAddr.Port
		}
		if serverURL.Path != "/dns-query" {
			httpsOptions.Path = serverURL.Path
		}
	case "rcode":
		var rcode int
		switch serverURL.Host {
		case "success":
			rcode = dns.RcodeSuccess
		case "format_error":
			rcode = dns.RcodeFormatError
		case "server_failure":
			rcode = dns.RcodeServerFailure
		case "name_error":
			rcode = dns.RcodeNameError
		case "not_implemented":
			rcode = dns.RcodeNotImplemented
		case "refused":
			rcode = dns.RcodeRefused
		default:
			return E.New("unknown rcode: ", serverURL.Host)
		}
		o.Type = C.DNSTypePreDefined
		o.Options = &PredefinedDNSServerOptions{
			Responses: []DNSResponseOptions{
				{
					RCode: common.Ptr(DNSRCode(rcode)),
				},
			},
		}
	case C.DNSTypeDHCP:
		o.Type = C.DNSTypeDHCP
		dhcpOptions := DHCPDNSServerOptions{}
		if serverURL.Host != "" && serverURL.Host != "auto" {
			dhcpOptions.Interface = serverURL.Host
		}
		o.Options = &dhcpOptions
	case C.DNSTypeFakeIP:
		o.Type = C.DNSTypeFakeIP
		fakeipOptions := FakeIPDNSServerOptions{}
		if legacyOptions, loaded := ctx.Value((*LegacyDNSFakeIPOptions)(nil)).(*LegacyDNSFakeIPOptions); loaded {
			fakeipOptions.Inet4Range = legacyOptions.Inet4Range
			fakeipOptions.Inet6Range = legacyOptions.Inet6Range
		}
		o.Options = &fakeipOptions
	default:
		return E.New("unsupported DNS server scheme: ", serverType)
	}
	return nil
}

type LegacyDNSServerOptions struct {
	Address              string                `json:"address"`
	AddressResolver      string                `json:"address_resolver,omitempty"`
	AddressStrategy      DomainStrategy        `json:"address_strategy,omitempty"`
	AddressFallbackDelay badoption.Duration    `json:"address_fallback_delay,omitempty"`
	Strategy             DomainStrategy        `json:"strategy,omitempty"`
	Detour               string                `json:"detour,omitempty"`
	ClientSubnet         *badoption.Prefixable `json:"client_subnet,omitempty"`
}

type HostsDNSServerOptions struct {
	Path       badoption.Listable[string]                               `json:"path,omitempty"`
	Predefined badjson.TypedMap[string, badoption.Listable[netip.Addr]] `json:"predefined,omitempty"`
}

type LocalDNSServerOptions struct {
	DialerOptions
	LegacyStrategy      DomainStrategy `json:"-"`
	LegacyDefaultDialer bool           `json:"-"`
	LegacyClientSubnet  netip.Prefix   `json:"-"`
}

type RemoteDNSServerOptions struct {
	LocalDNSServerOptions
	ServerOptions
	LegacyAddressResolver      string             `json:"-"`
	LegacyAddressStrategy      DomainStrategy     `json:"-"`
	LegacyAddressFallbackDelay badoption.Duration `json:"-"`
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
