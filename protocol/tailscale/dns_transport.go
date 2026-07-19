//go:build with_gvisor

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
	"github.com/sagernet/tailscale/util/dnsname"
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
	acceptSearchDomain     bool
	dnsRouter              adapter.DNSRouter
	endpointManager        adapter.EndpointManager
	endpoint               *Endpoint
	access                 sync.RWMutex
	routePrefixes          []netip.Prefix
	routes                 map[string][]adapter.DNSTransport
	hosts                  map[string][]netip.Addr
	searchDomains          []string
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
		acceptSearchDomain:     options.AcceptSearchDomain,
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
	t.access.RLock()
	transports := t.collectResolversLocked()
	t.access.RUnlock()
	for _, transport := range transports {
		transport.Reset()
	}
}

func (t *DNSTransport) onReconfig(cfg *wgcfg.Config, routerCfg *router.Config, dnsCfg *nDNS.Config) {
	err := t.updateDNSServers(routerCfg, dnsCfg)
	if err != nil {
		t.logger.Error(E.Cause(err, "update DNS servers"))
	}
}

func (t *DNSTransport) updateDNSServers(routeConfig *router.Config, dnsConfig *nDNS.Config) error {
	routePrefixes := buildRoutePrefixes(routeConfig)
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
	searchDomains := common.Map(dnsConfig.SearchDomains, func(it dnsname.FQDN) string {
		return it.WithTrailingDot()
	})
	var defaultResolvers []adapter.DNSTransport
	for _, resolver := range dnsConfig.DefaultResolvers {
		myResolver, err := t.createResolver(directDialerOnce, resolver)
		if err != nil {
			return err
		}
		defaultResolvers = append(defaultResolvers, myResolver)
	}

	t.access.Lock()
	oldResolvers := t.collectResolversLocked()
	t.routePrefixes = routePrefixes
	t.routes = routes
	t.hosts = hosts
	t.searchDomains = searchDomains
	t.defaultResolvers = defaultResolvers
	t.access.Unlock()

	for _, transport := range oldResolvers {
		transport.Close()
	}

	if len(defaultResolvers) > 0 {
		t.logger.Info("updated ", len(routes), " routes, ", len(hosts), " hosts, ", len(searchDomains), " search domains, default resolvers: ",
			strings.Join(common.Map(dnsConfig.DefaultResolvers, func(it *dnstype.Resolver) string { return it.Addr }), " "))
	} else {
		t.logger.Info("updated ", len(routes), " routes, ", len(hosts), " hosts, ", len(searchDomains), " search domains")
	}
	return nil
}

func (t *DNSTransport) createResolver(directDialer func() N.Dialer, resolver *dnstype.Resolver) (adapter.DNSTransport, error) {
	serverURL, parseURLErr := url.Parse(resolver.Addr)
	isHTTPScheme := parseURLErr == nil && (serverURL.Scheme == "http" || serverURL.Scheme == "https")
	var myDialer N.Dialer
	if isHTTPScheme && serverURL.Scheme == "http" {
		myDialer = t.endpoint
	} else {
		myDialer = directDialer()
	}
	if len(resolver.BootstrapResolution) > 0 {
		bootstrapTransport := transport.NewUDPRaw(t.logger, t.TransportAdapter, myDialer, M.SocksaddrFrom(resolver.BootstrapResolution[0], 53))
		myDialer = dialer.NewResolveDialer(t.ctx, myDialer, false, "", adapter.DNSQueryOptions{Transport: bootstrapTransport}, 0)
	} else {
		myDialer = dialer.NewResolveDialer(t.ctx, myDialer, false, "", t.endpoint.queryOptions, 0)
	}
	if isHTTPScheme {
		serverAddr := M.ParseSocksaddrHostPortStr(serverURL.Hostname(), serverURL.Port())
		switch serverURL.Scheme {
		case "https":
			if serverAddr.Port == 0 {
				serverAddr.Port = 443
			}
			tlsConfig := common.Must1(tls.NewClient(t.ctx, t.logger, serverAddr.AddrString(), option.OutboundTLSOptions{
				Enabled: true,
				ALPN:    []string{http2.NextProtoTLS, "http/1.1"},
			}))
			return transport.NewHTTPSRaw(t.TransportAdapter, t.logger, myDialer, serverURL, http.Header{}, serverAddr, tlsConfig), nil
		case "http":
			if serverAddr.Port == 0 {
				serverAddr.Port = 80
			}
			return transport.NewHTTPSRaw(t.TransportAdapter, t.logger, myDialer, serverURL, http.Header{}, serverAddr, nil), nil
		}
	}
	serverAddr := M.ParseSocksaddr(resolver.Addr)
	if !serverAddr.IsValid() {
		if parseURLErr != nil {
			return nil, E.Cause(parseURLErr, "parse resolver address")
		}
		return nil, E.New("invalid resolver address: ", resolver.Addr)
	}
	if serverAddr.Port == 0 {
		serverAddr.Port = 53
	}
	return transport.NewUDPRaw(t.logger, t.TransportAdapter, myDialer, serverAddr), nil
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
	t.access.Lock()
	transports := t.collectResolversLocked()
	t.routePrefixes = nil
	t.routes = nil
	t.hosts = nil
	t.defaultResolvers = nil
	t.access.Unlock()

	var err error
	for _, transport := range transports {
		name := "resolver/" + transport.Type() + "[" + transport.Tag() + "]"
		err = E.Append(err, transport.Close(), func(err error) error {
			return E.Cause(err, "close ", name)
		})
	}
	return err
}

func (t *DNSTransport) Raw() bool {
	return true
}

func (t *DNSTransport) PreferredDomain(domain string) bool {
	t.access.RLock()
	hosts := t.hosts
	routes := t.routes
	t.access.RUnlock()
	if _, loaded := hosts[domain]; loaded {
		return true
	}
	for suffix := range routes {
		if mDNS.IsSubDomain(suffix, domain) {
			return true
		}
	}
	return false
}

func (t *DNSTransport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	done := make(chan struct{})
	var (
		response *mDNS.Msg
		err      error
	)
	t.ExchangeAsync(ctx, message, func(callbackResponse *mDNS.Msg, callbackErr error) {
		response = callbackResponse
		err = callbackErr
		close(done)
	})
	<-done
	return response, err
}

func (t *DNSTransport) ExchangeAsync(ctx context.Context, message *mDNS.Msg, callback func(response *mDNS.Msg, err error)) {
	if len(message.Question) != 1 {
		callback(nil, os.ErrInvalid)
		return
	}
	if t.acceptSearchDomain && mDNS.CountLabel(message.Question[0].Name) == 1 {
		t.exchangeWithSearchDomains(ctx, message, callback)
		return
	}
	t.access.RLock()
	acceptDefaultResolvers := t.acceptDefaultResolvers
	t.access.RUnlock()
	t.exchangeOnce(ctx, message, acceptDefaultResolvers, callback)
}

func (t *DNSTransport) exchangeWithSearchDomains(ctx context.Context, message *mDNS.Msg, callback func(response *mDNS.Msg, err error)) {
	t.access.RLock()
	searchDomains := t.searchDomains
	t.access.RUnlock()
	if len(searchDomains) == 0 {
		callback(nil, dns.RcodeNameError)
		return
	}
	originalQuestion := message.Question[0]
	singleLabel := strings.TrimSuffix(originalQuestion.Name, ".")
	domainExchangers := make([]transport.AsyncExchanger, 0, len(searchDomains))
	for _, searchDomain := range searchDomains {
		expandedName := singleLabel + "." + searchDomain
		domainExchangers = append(domainExchangers, func(exchangeCtx context.Context, exchangeCallback func(response *mDNS.Msg, err error)) {
			question := originalQuestion
			question.Name = expandedName
			rewritten := *message
			rewritten.Question = []mDNS.Question{question}
			t.exchangeOnce(exchangeCtx, &rewritten, false, func(response *mDNS.Msg, err error) {
				if err == nil {
					restoreOriginalQuestion(response, expandedName, originalQuestion)
				}
				exchangeCallback(response, err)
			})
		})
	}
	transport.ExchangeSequential(ctx, domainExchangers, func(response *mDNS.Msg, err error) bool {
		return err == nil && response.Rcode != mDNS.RcodeNameError
	}, callback)
}

// RFC 1035 §4.1.1 requires the response Question to match the request byte-for-byte,
// and stub resolvers discard Answer RRs whose owner name does not match the question.
func restoreOriginalQuestion(response *mDNS.Msg, expandedName string, originalQuestion mDNS.Question) {
	response.Question = []mDNS.Question{originalQuestion}
	for _, rr := range response.Answer {
		if strings.EqualFold(rr.Header().Name, expandedName) {
			rr.Header().Name = originalQuestion.Name
		}
	}
}

func (t *DNSTransport) exchangeOnce(ctx context.Context, message *mDNS.Msg, allowDefaultResolvers bool, callback func(response *mDNS.Msg, err error)) {
	question := message.Question[0]

	t.access.RLock()
	hosts := t.hosts
	routes := t.routes
	defaultResolvers := t.defaultResolvers
	t.access.RUnlock()

	addresses, hostsLoaded := hosts[question.Name]
	if hostsLoaded {
		switch question.Qtype {
		case mDNS.TypeA:
			addresses4 := common.Filter(addresses, func(addr netip.Addr) bool {
				return addr.Is4()
			})
			if len(addresses4) > 0 {
				callback(dns.FixedResponse(message.Id, question, addresses4, C.DefaultDNSTTL), nil)
				return
			}
		case mDNS.TypeAAAA:
			addresses6 := common.Filter(addresses, func(addr netip.Addr) bool {
				return addr.Is6()
			})
			if len(addresses6) > 0 {
				callback(dns.FixedResponse(message.Id, question, addresses6, C.DefaultDNSTTL), nil)
				return
			}
		}
	}
	for domainSuffix, transports := range routes {
		if mDNS.IsSubDomain(domainSuffix, question.Name) {
			if len(transports) == 0 {
				callback(&mDNS.Msg{
					MsgHdr: mDNS.MsgHdr{
						Id:       message.Id,
						Rcode:    mDNS.RcodeNameError,
						Response: true,
					},
					Question: []mDNS.Question{question},
				}, nil)
				return
			}
			transport.ExchangeSequential(ctx, resolverExchangers(transports, message), nil, callback)
			return
		}
	}
	if allowDefaultResolvers {
		if len(defaultResolvers) > 0 {
			transport.ExchangeSequential(ctx, resolverExchangers(defaultResolvers, message), nil, callback)
		} else {
			callback(nil, E.New("missing default resolvers"))
		}
		return
	}
	callback(nil, dns.RcodeNameError)
}

func resolverExchangers(resolvers []adapter.DNSTransport, message *mDNS.Msg) []transport.AsyncExchanger {
	return common.Map(resolvers, func(resolver adapter.DNSTransport) transport.AsyncExchanger {
		return func(ctx context.Context, callback func(response *mDNS.Msg, err error)) {
			resolver.ExchangeAsync(ctx, message, callback)
		}
	})
}

func (t *DNSTransport) collectResolversLocked() []adapter.DNSTransport {
	var transports []adapter.DNSTransport
	for _, resolvers := range t.routes {
		transports = append(transports, resolvers...)
	}
	transports = append(transports, t.defaultResolvers...)
	return transports
}

type DNSDialer struct {
	transport      *DNSTransport
	fallbackDialer N.Dialer
}

func (d *DNSDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if destination.IsDomain() {
		panic("invalid request here")
	}
	routePrefixes := d.transport.routePrefixesSnapshot()
	for _, prefix := range routePrefixes {
		if prefix.Contains(destination.Addr) {
			return d.transport.endpoint.DialContext(ctx, network, destination)
		}
	}
	return d.fallbackDialer.DialContext(ctx, network, destination)
}

func (d *DNSDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	if destination.IsDomain() {
		panic("invalid request here")
	}
	routePrefixes := d.transport.routePrefixesSnapshot()
	for _, prefix := range routePrefixes {
		if prefix.Contains(destination.Addr) {
			return d.transport.endpoint.ListenPacket(ctx, destination)
		}
	}
	return d.fallbackDialer.ListenPacket(ctx, destination)
}

func (t *DNSTransport) routePrefixesSnapshot() []netip.Prefix {
	t.access.RLock()
	defer t.access.RUnlock()
	return append([]netip.Prefix(nil), t.routePrefixes...)
}
