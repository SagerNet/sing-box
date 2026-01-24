package tailscale

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/dns/transport"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
	nDNS "github.com/sagernet/tailscale/net/dns"
	"github.com/sagernet/tailscale/types/dnstype"
	"github.com/sagernet/tailscale/wgengine/router"
	"github.com/sagernet/tailscale/wgengine/wgcfg"

	mDNS "github.com/miekg/dns"
	"go4.org/netipx"
	"golang.org/x/net/http2"
)

func RegistryTransport(registry *dns.TransportRegistry) {
	dns.RegisterTransport[option.TailscaleDNSServerOptions](registry, C.DNSTypeTailscale, NewDNSTransport)
}

type DNSTransport struct {
	dns.TransportAdapter
	ctx                    context.Context
	logger                 logger.ContextLogger
	endpointTag            string
	acceptDefaultResolvers bool
	dnsRouter              adapter.DNSRouter
	endpointManager        adapter.EndpointManager
	endpoint               *Endpoint
	routePrefixes          []netip.Prefix
	routes                 map[string][]adapter.DNSTransport
	hosts                  map[string][]netip.Addr
	defaultResolvers       []adapter.DNSTransport
}

func NewDNSTransport(ctx context.Context, logger log.ContextLogger, tag string, options option.TailscaleDNSServerOptions) (adapter.DNSTransport, error) {
	if options.Endpoint == "" {
		return nil, E.New("missing tailscale endpoint tag")
	}
	return &DNSTransport{
		TransportAdapter:       dns.NewTransportAdapter(C.DNSTypeTailscale, tag, nil),
		ctx:                    ctx,
		logger:                 logger,
		endpointTag:            options.Endpoint,
		acceptDefaultResolvers: options.AcceptDefaultResolvers,
		dnsRouter:              service.FromContext[adapter.DNSRouter](ctx),
		endpointManager:        service.FromContext[adapter.EndpointManager](ctx),
	}, nil
}

func (t *DNSTransport) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateInitialize {
		return nil
	}
	rawOutbound, loaded := t.endpointManager.Get(t.endpointTag)
	if !loaded {
		return E.New("endpoint not found: ", t.endpointTag)
	}
	ep, isTailscale := rawOutbound.(*Endpoint)
	if !isTailscale {
		return E.New("endpoint is not Tailscale: ", t.endpointTag)
	}
	if ep.onReconfigHook != nil {
		return E.New("only one Tailscale DNS server is allowed for single endpoint")
	}
	ep.onReconfigHook = t.onReconfig
	t.endpoint = ep
	return nil
}

func (t *DNSTransport) Reset() {
}

func (t *DNSTransport) onReconfig(cfg *wgcfg.Config, routerCfg *router.Config, dnsCfg *nDNS.Config) {
	err := t.updateDNSServers(routerCfg, dnsCfg)
	if err != nil {
		t.logger.Error(E.Cause(err, "update DNS servers"))
	}
}

func (t *DNSTransport) updateDNSServers(routeConfig *router.Config, dnsConfig *nDNS.Config) error {
	t.routePrefixes = buildRoutePrefixes(routeConfig)
	directDialerOnce := sync.OnceValue(func() N.Dialer {
		directDialer := common.Must1(dialer.NewDefault(t.ctx, option.DialerOptions{}))
		return &DNSDialer{transport: t, fallbackDialer: directDialer}
	})
	routes := make(map[string][]adapter.DNSTransport)
	for domain, resolvers := range dnsConfig.Routes {
		var myResolvers []adapter.DNSTransport
		for _, resolver := range resolvers {
			myResolver, err := t.createResolver(directDialerOnce, resolver)
			if err != nil {
				return err
			}
			myResolvers = append(myResolvers, myResolver)
		}
		routes[domain.WithTrailingDot()] = myResolvers
	}
	hosts := make(map[string][]netip.Addr)
	for domain, addresses := range dnsConfig.Hosts {
		hosts[domain.WithTrailingDot()] = addresses
	}
	var defaultResolvers []adapter.DNSTransport
	for _, resolver := range dnsConfig.DefaultResolvers {
		myResolver, err := t.createResolver(directDialerOnce, resolver)
		if err != nil {
			return err
		}
		defaultResolvers = append(defaultResolvers, myResolver)
	}
	t.routes = routes
	t.hosts = hosts
	t.defaultResolvers = defaultResolvers
	if len(defaultResolvers) > 0 {
		t.logger.Info("updated ", len(routes), " routes, ", len(hosts), " hosts, default resolvers: ",
			strings.Join(common.Map(dnsConfig.DefaultResolvers, func(it *dnstype.Resolver) string { return it.Addr }), " "))
	} else {
		t.logger.Info("updated ", len(routes), " routes, ", len(hosts), " hosts")
	}
	return nil
}

func (t *DNSTransport) createResolver(directDialer func() N.Dialer, resolver *dnstype.Resolver) (adapter.DNSTransport, error) {
	serverURL, parseURLErr := url.Parse(resolver.Addr)
	var myDialer N.Dialer
	if parseURLErr == nil && serverURL.Scheme == "http" {
		myDialer = t.endpoint
	} else {
		myDialer = directDialer()
	}
	if len(resolver.BootstrapResolution) > 0 {
		bootstrapTransport := transport.NewUDPRaw(t.logger, t.TransportAdapter, myDialer, M.SocksaddrFrom(resolver.BootstrapResolution[0], 53))
		myDialer = dialer.NewResolveDialer(t.ctx, myDialer, false, "", adapter.DNSQueryOptions{Transport: bootstrapTransport}, 0)
	}
	if serverAddr := M.ParseSocksaddr(resolver.Addr); serverAddr.IsValid() {
		if serverAddr.Port == 0 {
			serverAddr.Port = 53
		}
		return transport.NewUDPRaw(t.logger, t.TransportAdapter, myDialer, serverAddr), nil
	} else if parseURLErr != nil {
		return nil, E.Cause(parseURLErr, "parse resolver address")
	} else {
		switch serverURL.Scheme {
		case "https":
			serverAddr = M.ParseSocksaddrHostPortStr(serverURL.Hostname(), serverURL.Port())
			if serverAddr.Port == 0 {
				serverAddr.Port = 443
			}
			tlsConfig := common.Must1(tls.NewClient(t.ctx, t.logger, serverAddr.AddrString(), option.OutboundTLSOptions{
				ALPN: []string{http2.NextProtoTLS, "http/1.1"},
			}))
			return transport.NewHTTPSRaw(t.TransportAdapter, t.logger, myDialer, serverURL, http.Header{}, serverAddr, tlsConfig), nil
		case "http":
			serverAddr = M.ParseSocksaddrHostPortStr(serverURL.Hostname(), serverURL.Port())
			if serverAddr.Port == 0 {
				serverAddr.Port = 80
			}
			return transport.NewHTTPSRaw(t.TransportAdapter, t.logger, myDialer, serverURL, http.Header{}, serverAddr, nil), nil
		// case "tls":
		default:
			return nil, E.New("unknown resolver scheme: ", serverURL.Scheme)
		}
	}
}

func buildRoutePrefixes(routeConfig *router.Config) []netip.Prefix {
	var builder netipx.IPSetBuilder
	for _, localAddr := range routeConfig.LocalAddrs {
		builder.AddPrefix(localAddr)
	}
	for _, route := range routeConfig.Routes {
		builder.AddPrefix(route)
	}
	for _, route := range routeConfig.LocalRoutes {
		builder.AddPrefix(route)
	}
	for _, route := range routeConfig.SubnetRoutes {
		builder.AddPrefix(route)
	}
	ipSet, err := builder.IPSet()
	if err != nil {
		return nil
	}
	return ipSet.Prefixes()
}

func (t *DNSTransport) Close() error {
	return nil
}

func (t *DNSTransport) Raw() bool {
	return true
}

func (t *DNSTransport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	if len(message.Question) != 1 {
		return nil, os.ErrInvalid
	}
	question := message.Question[0]
	addresses, hostsLoaded := t.hosts[question.Name]
	if hostsLoaded {
		switch question.Qtype {
		case mDNS.TypeA:
			addresses4 := common.Filter(addresses, func(addr netip.Addr) bool {
				return addr.Is4()
			})
			if len(addresses4) > 0 {
				return dns.FixedResponse(message.Id, question, addresses4, C.DefaultDNSTTL), nil
			}
		case mDNS.TypeAAAA:
			addresses6 := common.Filter(addresses, func(addr netip.Addr) bool {
				return addr.Is6()
			})
			if len(addresses6) > 0 {
				return dns.FixedResponse(message.Id, question, addresses6, C.DefaultDNSTTL), nil
			}
		}
	}
	for domainSuffix, transports := range t.routes {
		if strings.HasSuffix(question.Name, domainSuffix) {
			if len(transports) == 0 {
				return &mDNS.Msg{
					MsgHdr: mDNS.MsgHdr{
						Id:       message.Id,
						Rcode:    mDNS.RcodeNameError,
						Response: true,
					},
					Question: []mDNS.Question{question},
				}, nil
			}
			var lastErr error
			for _, dnsTransport := range transports {
				response, err := dnsTransport.Exchange(ctx, message)
				if err != nil {
					lastErr = err
					continue
				}
				return response, nil
			}
			return nil, lastErr
		}
	}
	if t.acceptDefaultResolvers {
		if len(t.defaultResolvers) > 0 {
			var lastErr error
			for _, resolver := range t.defaultResolvers {
				response, err := resolver.Exchange(ctx, message)
				if err != nil {
					lastErr = err
					continue
				}
				return response, nil
			}
			return nil, lastErr
		} else {
			return nil, E.New("missing default resolvers")
		}
	}
	return nil, dns.RcodeNameError
}

type DNSDialer struct {
	transport      *DNSTransport
	fallbackDialer N.Dialer
}

func (d *DNSDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if destination.IsFqdn() {
		panic("invalid request here")
	}
	for _, prefix := range d.transport.routePrefixes {
		if prefix.Contains(destination.Addr) {
			return d.transport.endpoint.DialContext(ctx, network, destination)
		}
	}
	return d.fallbackDialer.DialContext(ctx, network, destination)
}

func (d *DNSDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	if destination.IsFqdn() {
		panic("invalid request here")
	}
	for _, prefix := range d.transport.routePrefixes {
		if prefix.Contains(destination.Addr) {
			return d.transport.endpoint.ListenPacket(ctx, destination)
		}
	}
	return d.fallbackDialer.ListenPacket(ctx, destination)
}
