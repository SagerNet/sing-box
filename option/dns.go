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
	Servers        []DNSServerOptions `json:"servers,omitempty"`
	Rules          []DNSRule          `json:"rules,omitempty"`
	Final          string             `json:"final,omitempty"`
	ReverseMapping bool               `json:"reverse_mapping,omitempty"`
	DNSClientOptions
}

type LegacyDNSOptions struct {
	FakeIP *LegacyDNSFakeIPOptions `json:"fakeip,omitempty"`
}

type DNSOptions struct {
	RawDNSOptions
	LegacyDNSOptions
}

type contextKeyDontUpgrade struct{}

func ContextWithDontUpgrade(ctx context.Context) context.Context {
	return context.WithValue(ctx, (*contextKeyDontUpgrade)(nil), true)
}

func dontUpgradeFromContext(ctx context.Context) bool {
	return ctx.Value((*contextKeyDontUpgrade)(nil)) == true
}

func (o *DNSOptions) UnmarshalJSONContext(ctx context.Context, content []byte) error {
	err := json.UnmarshalContext(ctx, content, &o.LegacyDNSOptions)
	if err != nil {
		return err
	}
	dontUpgrade := dontUpgradeFromContext(ctx)
	legacyOptions := o.LegacyDNSOptions
	if !dontUpgrade {
		if o.FakeIP != nil && o.FakeIP.Enabled {
			deprecated.Report(ctx, deprecated.OptionLegacyDNSFakeIPOptions)
			ctx = context.WithValue(ctx, (*LegacyDNSFakeIPOptions)(nil), o.FakeIP)
		}
		o.LegacyDNSOptions = LegacyDNSOptions{}
	}
	err = badjson.UnmarshallExcludedContext(ctx, content, legacyOptions, &o.RawDNSOptions)
	if err != nil {
		return err
	}
	if !dontUpgrade {
		rcodeMap := make(map[string]int)
		o.Servers = common.Filter(o.Servers, func(it DNSServerOptions) bool {
			if it.Type == C.DNSTypeLegacyRcode {
				rcodeMap[it.Tag] = it.Options.(int)
				return false
			}
			return true
		})
		if len(rcodeMap) > 0 {
			for i := 0; i < len(o.Rules); i++ {
				rewriteRcode(rcodeMap, &o.Rules[i])
			}
		}
	}
	return nil
}

func rewriteRcode(rcodeMap map[string]int, rule *DNSRule) {
	switch rule.Type {
	case C.RuleTypeDefault:
		rewriteRcodeAction(rcodeMap, &rule.DefaultOptions.DNSRuleAction)
	case C.RuleTypeLogical:
		rewriteRcodeAction(rcodeMap, &rule.LogicalOptions.DNSRuleAction)
	}
}

func rewriteRcodeAction(rcodeMap map[string]int, ruleAction *DNSRuleAction) {
	if ruleAction.Action != C.RuleActionTypeRoute {
		return
	}
	rcode, loaded := rcodeMap[ruleAction.RouteOptions.Server]
	if !loaded {
		return
	}
	ruleAction.Action = C.RuleActionTypePredefined
	ruleAction.PredefinedOptions.Rcode = common.Ptr(DNSRCode(rcode))
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
type _DNSServerOptions struct {
	Type    string `json:"type,omitempty"`
	Tag     string `json:"tag,omitempty"`
	Options any    `json:"-"`
}

type DNSServerOptions _DNSServerOptions

func (o *DNSServerOptions) MarshalJSONContext(ctx context.Context) ([]byte, error) {
	switch o.Type {
	case C.DNSTypeLegacy:
		o.Type = ""
	}
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
	err = badjson.UnmarshallExcludedContext(ctx, content, (*_DNSServerOptions)(o), options)
	if err != nil {
		return err
	}
	o.Options = options
	if o.Type == C.DNSTypeLegacy && !dontUpgradeFromContext(ctx) {
		err = o.Upgrade(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *DNSServerOptions) Upgrade(ctx context.Context) error {
	if o.Type != C.DNSTypeLegacy {
		return nil
	}
	options := o.Options.(*LegacyDNSServerOptions)
	serverURL, _ := url.Parse(options.Address)
	var serverType string
	if serverURL != nil && serverURL.Scheme != "" {
		serverType = serverURL.Scheme
	} else {
		switch options.Address {
		case "local", "fakeip":
			serverType = options.Address
		default:
			serverType = C.DNSTypeUDP
		}
	}
	remoteOptions := RemoteDNSServerOptions{
		RawLocalDNSServerOptions: RawLocalDNSServerOptions{
			DialerOptions: DialerOptions{
				Detour: options.Detour,
				DomainResolver: &DomainResolveOptions{
					Server:   options.AddressResolver,
					Strategy: options.AddressStrategy,
				},
				FallbackDelay: options.AddressFallbackDelay,
			},
			Legacy:              true,
			LegacyStrategy:      options.Strategy,
			LegacyDefaultDialer: options.Detour == "",
			LegacyClientSubnet:  options.ClientSubnet.Build(netip.Prefix{}),
		},
		LegacyAddressResolver:      options.AddressResolver,
		LegacyAddressStrategy:      options.AddressStrategy,
		LegacyAddressFallbackDelay: options.AddressFallbackDelay,
	}
	switch serverType {
	case C.DNSTypeLocal:
		o.Type = C.DNSTypeLocal
		o.Options = &LocalDNSServerOptions{
			RawLocalDNSServerOptions: remoteOptions.RawLocalDNSServerOptions,
		}
	case C.DNSTypeUDP:
		o.Type = C.DNSTypeUDP
		o.Options = &remoteOptions
		var serverAddr M.Socksaddr
		if serverURL == nil || serverURL.Scheme == "" {
			serverAddr = M.ParseSocksaddr(options.Address)
		} else {
			serverAddr = M.ParseSocksaddr(serverURL.Host)
		}
		if !serverAddr.IsValid() {
			return E.New("invalid server address")
		}
		remoteOptions.Server = serverAddr.AddrString()
		if serverAddr.Port != 0 && serverAddr.Port != 53 {
			remoteOptions.ServerPort = serverAddr.Port
		}
	case C.DNSTypeTCP:
		o.Type = C.DNSTypeTCP
		o.Options = &remoteOptions
		if serverURL == nil {
			return E.New("invalid server address")
		}
		serverAddr := M.ParseSocksaddr(serverURL.Host)
		if !serverAddr.IsValid() {
			return E.New("invalid server address")
		}
		remoteOptions.Server = serverAddr.AddrString()
		if serverAddr.Port != 0 && serverAddr.Port != 53 {
			remoteOptions.ServerPort = serverAddr.Port
		}
	case C.DNSTypeTLS, C.DNSTypeQUIC:
		o.Type = serverType
		if serverURL == nil {
			return E.New("invalid server address")
		}
		serverAddr := M.ParseSocksaddr(serverURL.Host)
		if !serverAddr.IsValid() {
			return E.New("invalid server address")
		}
		remoteOptions.Server = serverAddr.AddrString()
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
		if serverURL == nil {
			return E.New("invalid server address")
		}
		serverAddr := M.ParseSocksaddr(serverURL.Host)
		if !serverAddr.IsValid() {
			return E.New("invalid server address")
		}
		httpsOptions.Server = serverAddr.AddrString()
		if serverAddr.Port != 0 && serverAddr.Port != 443 {
			httpsOptions.ServerPort = serverAddr.Port
		}
		if serverURL.Path != "/dns-query" {
			httpsOptions.Path = serverURL.Path
		}
	case "rcode":
		var rcode int
		if serverURL == nil {
			return E.New("invalid server address")
		}
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
		o.Type = C.DNSTypeLegacyRcode
		o.Options = rcode
	case C.DNSTypeDHCP:
		o.Type = C.DNSTypeDHCP
		dhcpOptions := DHCPDNSServerOptions{}
		if serverURL == nil {
			return E.New("invalid server address")
		}
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

type DNSServerAddressOptions struct {
	Server     string `json:"server"`
	ServerPort uint16 `json:"server_port,omitempty"`
}

func (o DNSServerAddressOptions) Build() M.Socksaddr {
	return M.ParseSocksaddrHostPort(o.Server, o.ServerPort)
}

func (o DNSServerAddressOptions) ServerIsDomain() bool {
	return M.IsDomainName(o.Server)
}

func (o *DNSServerAddressOptions) TakeServerOptions() ServerOptions {
	return ServerOptions(*o)
}

func (o *DNSServerAddressOptions) ReplaceServerOptions(options ServerOptions) {
	*o = DNSServerAddressOptions(options)
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
	Path       badoption.Listable[string]                                `json:"path,omitempty"`
	Predefined *badjson.TypedMap[string, badoption.Listable[netip.Addr]] `json:"predefined,omitempty"`
}

type RawLocalDNSServerOptions struct {
	DialerOptions
	Legacy              bool           `json:"-"`
	LegacyStrategy      DomainStrategy `json:"-"`
	LegacyDefaultDialer bool           `json:"-"`
	LegacyClientSubnet  netip.Prefix   `json:"-"`
}

type LocalDNSServerOptions struct {
	RawLocalDNSServerOptions
	PreferGo bool `json:"prefer_go,omitempty"`
}

type RemoteDNSServerOptions struct {
	RawLocalDNSServerOptions
	DNSServerAddressOptions
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
