package openvpn

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	boxTLS "github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	boxDNS "github.com/sagernet/sing-box/dns"
	dnsTransport "github.com/sagernet/sing-box/dns/transport"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	ovpntransport "github.com/sagernet/sing-box/transport/openvpn"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"

	mDNS "github.com/miekg/dns"
	"golang.org/x/net/http2"
)

func RegisterDNSTransport(registry *boxDNS.TransportRegistry) {
	boxDNS.RegisterTransport[option.OpenVPNDNSServerOptions](registry, C.DNSTypeOpenVPN, NewDNSTransport)
}

type DNSTransport struct {
	boxDNS.TransportAdapter
	ctx                    context.Context
	logger                 logger.ContextLogger
	endpointTag            string
	acceptDefaultResolvers bool
	acceptSearchDomain     bool
	endpointManager        adapter.EndpointManager
	endpoint               *ClientEndpoint
	dialer                 N.Dialer
	updateAccess           sync.Mutex
	access                 sync.RWMutex
	closed                 bool
	routes                 map[string][]adapter.DNSTransport
	searchDomains          []string
	defaultResolvers       []adapter.DNSTransport
}

func NewDNSTransport(ctx context.Context, logger log.ContextLogger, tag string, options option.OpenVPNDNSServerOptions) (adapter.DNSTransport, error) {
	if options.Endpoint == "" {
		return nil, E.New("missing endpoint tag")
	}
	return &DNSTransport{
		TransportAdapter:       boxDNS.NewTransportAdapter(C.DNSTypeOpenVPN, tag, nil),
		ctx:                    ctx,
		logger:                 logger,
		endpointTag:            options.Endpoint,
		acceptDefaultResolvers: options.AcceptDefaultResolvers,
		acceptSearchDomain:     options.AcceptSearchDomain,
		endpointManager:        service.FromContext[adapter.EndpointManager](ctx),
	}, nil
}

func (t *DNSTransport) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateInitialize {
		return nil
	}
	rawEndpoint, loaded := t.endpointManager.Get(t.endpointTag)
	if !loaded {
		return E.New("endpoint not found: ", t.endpointTag)
	}
	endpoint, isOpenVPN := rawEndpoint.(*ClientEndpoint)
	if !isOpenVPN {
		return E.New("endpoint is not an OpenVPN client: ", t.endpointTag)
	}
	t.endpoint = endpoint
	t.dialer = endpoint
	err := endpoint.installDNSTransport(t)
	if err != nil {
		t.endpoint = nil
		t.dialer = nil
		return err
	}
	return nil
}

func (t *DNSTransport) onReconfiguration(configuration ovpntransport.Configuration) {
	err := t.updateResolvers(configuration)
	if err != nil && !E.IsClosed(err) {
		t.logger.Error(E.Cause(err, "update DNS resolvers"))
	}
}

func (t *DNSTransport) updateResolvers(configuration ovpntransport.Configuration) error {
	t.updateAccess.Lock()
	defer t.updateAccess.Unlock()
	t.access.RLock()
	closed := t.closed
	t.access.RUnlock()
	if closed {
		return net.ErrClosed
	}
	routes := make(map[string][]adapter.DNSTransport)
	searchDomains := normalizeOpenVPNDomains(configuration.SearchDomains)
	var defaultResolvers []adapter.DNSTransport
	var newResolvers []adapter.DNSTransport
	servers := slices.Clone(configuration.DNSServers)
	slices.SortFunc(servers, func(left ovpntransport.DNSServer, right ovpntransport.DNSServer) int {
		return left.Priority - right.Priority
	})
	var selectedResolvers []adapter.DNSTransport
	if len(servers) > 0 {
		server := servers[0]
		if server.DNSSEC == "yes" {
			return t.failResolverUpdate(newResolvers, E.New("DNSSEC validation is required but is not supported"))
		}
		for _, address := range server.Addresses {
			resolver, err := t.createResolver(server, address)
			if err != nil {
				return t.failResolverUpdate(newResolvers, err)
			}
			selectedResolvers = append(selectedResolvers, resolver)
			newResolvers = append(newResolvers, resolver)
		}
		if len(selectedResolvers) == 0 {
			return t.failResolverUpdate(newResolvers, E.New("DNS server ", server.Priority, " has no addresses"))
		}
		if len(server.ResolveDomains) == 0 {
			defaultResolvers = slices.Clone(selectedResolvers)
		} else {
			for _, domain := range server.ResolveDomains {
				normalizedDomain := normalizeOpenVPNDomain(domain)
				if normalizedDomain != "" {
					routes[normalizedDomain] = slices.Clone(selectedResolvers)
				}
			}
		}
	} else {
		for _, address := range configuration.DNS {
			resolver := dnsTransport.NewUDPRaw(t.logger, t.TransportAdapter, t.dialer, M.SocksaddrFrom(address, 53))
			selectedResolvers = append(selectedResolvers, resolver)
			newResolvers = append(newResolvers, resolver)
		}
		if len(configuration.DNSRoutes) > 0 {
			if len(selectedResolvers) == 0 {
				return t.failResolverUpdate(newResolvers, E.New("DOMAIN-ROUTE requires traditional pushed DNS servers"))
			}
			for _, domain := range configuration.DNSRoutes {
				normalizedDomain := normalizeOpenVPNDomain(domain)
				if normalizedDomain != "" {
					routes[normalizedDomain] = slices.Clone(selectedResolvers)
				}
			}
		} else {
			defaultResolvers = slices.Clone(selectedResolvers)
		}
	}
	if len(searchDomains) > 0 && len(selectedResolvers) == 0 {
		return t.failResolverUpdate(newResolvers, E.New("search domains require pushed DNS servers"))
	}
	for _, searchDomain := range searchDomains {
		routes[searchDomain] = slices.Clone(selectedResolvers)
	}

	t.access.Lock()
	oldResolvers := t.collectResolversLocked()
	t.routes = routes
	t.searchDomains = searchDomains
	t.defaultResolvers = defaultResolvers
	t.access.Unlock()
	closeErr := closeDNSTransports(oldResolvers)
	t.logger.Info("updated ", len(routes), " DNS routes, ", len(searchDomains), " search domains and ", len(defaultResolvers), " default resolvers")
	return closeErr
}

func (t *DNSTransport) failResolverUpdate(newResolvers []adapter.DNSTransport, updateErr error) error {
	newCloseErr := closeDNSTransports(newResolvers)
	t.access.Lock()
	oldResolvers := t.collectResolversLocked()
	t.routes = nil
	t.searchDomains = nil
	t.defaultResolvers = nil
	t.access.Unlock()
	oldCloseErr := closeDNSTransports(oldResolvers)
	return E.Errors(updateErr, newCloseErr, oldCloseErr)
}

func (t *DNSTransport) createResolver(server ovpntransport.DNSServer, address netip.AddrPort) (adapter.DNSTransport, error) {
	transportType := strings.ToLower(server.Transport)
	if transportType == "" {
		transportType = "plain"
	}
	port := address.Port()
	switch transportType {
	case "plain":
		if port == 0 {
			port = 53
		}
		return dnsTransport.NewUDPRaw(t.logger, t.TransportAdapter, t.dialer, M.SocksaddrFrom(address.Addr(), port)), nil
	case "dot", "doh":
	default:
		return nil, E.New("unsupported DNS transport: ", server.Transport)
	}
	serverName := server.SNI
	if serverName == "" {
		serverName = address.Addr().String()
	}
	if transportType == "dot" {
		if port == 0 {
			port = 853
		}
		tlsConfig, err := boxTLS.NewClient(t.ctx, t.logger, serverName, option.OutboundTLSOptions{
			Enabled:    true,
			ServerName: serverName,
		})
		if err != nil {
			return nil, err
		}
		return dnsTransport.NewTLSRaw(t.logger, t.TransportAdapter, t.dialer, M.SocksaddrFrom(address.Addr(), port), tlsConfig), nil
	}
	if port == 0 {
		port = 443
	}
	tlsConfig, err := boxTLS.NewClient(t.ctx, t.logger, serverName, option.OutboundTLSOptions{
		Enabled:    true,
		ServerName: serverName,
		ALPN:       []string{http2.NextProtoTLS, "http/1.1"},
	})
	if err != nil {
		return nil, err
	}
	host := serverName
	if port != 443 {
		host = net.JoinHostPort(host, strconv.Itoa(int(port)))
	} else if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	destination := &url.URL{Scheme: "https", Host: host, Path: "/dns-query"}
	return dnsTransport.NewHTTPSRaw(t.TransportAdapter, t.logger, t.dialer, destination, http.Header{}, M.SocksaddrFrom(address.Addr(), port), tlsConfig), nil
}

func (t *DNSTransport) Reset() {
	t.access.RLock()
	resolvers := t.collectResolversLocked()
	t.access.RUnlock()
	for _, resolver := range resolvers {
		resolver.Reset()
	}
}

func (t *DNSTransport) Close() error {
	if t.endpoint != nil {
		t.endpoint.uninstallDNSTransport(t)
	}
	t.updateAccess.Lock()
	t.access.Lock()
	resolvers := t.collectResolversLocked()
	t.closed = true
	t.routes = nil
	t.searchDomains = nil
	t.defaultResolvers = nil
	t.access.Unlock()
	t.endpoint = nil
	t.dialer = nil
	t.updateAccess.Unlock()
	return closeDNSTransports(resolvers)
}

func (t *DNSTransport) Raw() bool {
	return true
}

func (t *DNSTransport) PreferredDomain(domain string) bool {
	t.access.RLock()
	defer t.access.RUnlock()
	for route := range t.routes {
		if openVPNDomainMatches(route, domain) {
			return true
		}
	}
	return false
}

func (t *DNSTransport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	done := make(chan struct{})
	var response *mDNS.Msg
	var err error
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
	t.access.RLock()
	searchDomains := slices.Clone(t.searchDomains)
	t.access.RUnlock()
	if t.acceptSearchDomain && len(searchDomains) > 0 && mDNS.CountLabel(message.Question[0].Name) == 1 {
		t.exchangeWithSearchDomains(ctx, message, searchDomains, callback)
		return
	}
	t.exchangeOnce(ctx, message, t.acceptDefaultResolvers, callback)
}

func (t *DNSTransport) exchangeWithSearchDomains(ctx context.Context, message *mDNS.Msg, searchDomains []string, callback func(response *mDNS.Msg, err error)) {
	originalQuestion := message.Question[0]
	singleLabel := strings.TrimSuffix(originalQuestion.Name, ".")
	exchangers := make([]dnsTransport.AsyncExchanger, 0, len(searchDomains)+1)
	for _, searchDomain := range searchDomains {
		expandedName := singleLabel + "." + searchDomain
		exchangers = append(exchangers, func(exchangeCtx context.Context, exchangeCallback func(response *mDNS.Msg, err error)) {
			question := originalQuestion
			question.Name = expandedName
			rewritten := *message
			rewritten.Question = []mDNS.Question{question}
			t.exchangeOnce(exchangeCtx, &rewritten, false, func(response *mDNS.Msg, err error) {
				if err == nil {
					restoreOpenVPNOriginalQuestion(response, expandedName, originalQuestion)
				}
				exchangeCallback(response, err)
			})
		})
	}
	exchangers = append(exchangers, func(exchangeCtx context.Context, exchangeCallback func(response *mDNS.Msg, err error)) {
		t.exchangeOnce(exchangeCtx, message, t.acceptDefaultResolvers, exchangeCallback)
	})
	dnsTransport.ExchangeSequential(ctx, exchangers, func(response *mDNS.Msg, err error) bool {
		return err == nil && response.Rcode != mDNS.RcodeNameError
	}, callback)
}

func (t *DNSTransport) exchangeOnce(ctx context.Context, message *mDNS.Msg, allowDefaultResolvers bool, callback func(response *mDNS.Msg, err error)) {
	question := message.Question[0]
	t.access.RLock()
	var matchedResolvers []adapter.DNSTransport
	matchedLength := -1
	for route, resolvers := range t.routes {
		if openVPNDomainMatches(route, question.Name) && len(route) > matchedLength {
			matchedLength = len(route)
			matchedResolvers = resolvers
		}
	}
	defaultResolvers := slices.Clone(t.defaultResolvers)
	t.access.RUnlock()
	if len(matchedResolvers) > 0 {
		dnsTransport.ExchangeSequential(ctx, openVPNResolverExchangers(matchedResolvers, message), nil, callback)
		return
	}
	if allowDefaultResolvers && len(defaultResolvers) > 0 {
		dnsTransport.ExchangeSequential(ctx, openVPNResolverExchangers(defaultResolvers, message), nil, callback)
		return
	}
	callback(nil, boxDNS.RcodeNameError)
}

func openVPNResolverExchangers(resolvers []adapter.DNSTransport, message *mDNS.Msg) []dnsTransport.AsyncExchanger {
	return common.Map(resolvers, func(resolver adapter.DNSTransport) dnsTransport.AsyncExchanger {
		return func(ctx context.Context, callback func(response *mDNS.Msg, err error)) {
			resolver.ExchangeAsync(ctx, message, callback)
		}
	})
}

func (t *DNSTransport) collectResolversLocked() []adapter.DNSTransport {
	var resolvers []adapter.DNSTransport
	for _, routeResolvers := range t.routes {
		resolvers = append(resolvers, routeResolvers...)
	}
	resolvers = append(resolvers, t.defaultResolvers...)
	return common.Uniq(resolvers)
}

func closeDNSTransports(resolvers []adapter.DNSTransport) error {
	var err error
	for _, resolver := range common.Uniq(resolvers) {
		err = E.Append(err, resolver.Close(), func(closeErr error) error {
			return E.Cause(closeErr, "close DNS resolver")
		})
	}
	return err
}

func normalizeOpenVPNDomain(domain string) string {
	normalized := strings.TrimSpace(strings.ToLower(domain))
	if normalized == "." {
		return normalized
	}
	normalized = strings.TrimSuffix(normalized, ".")
	if normalized == "" {
		return ""
	}
	return normalized + "."
}

func normalizeOpenVPNDomains(domains []string) []string {
	normalized := make([]string, 0, len(domains))
	for _, domain := range domains {
		normalizedDomain := normalizeOpenVPNDomain(domain)
		if normalizedDomain != "" && normalizedDomain != "." && !slices.Contains(normalized, normalizedDomain) {
			normalized = append(normalized, normalizedDomain)
		}
	}
	return normalized
}

func restoreOpenVPNOriginalQuestion(response *mDNS.Msg, expandedName string, originalQuestion mDNS.Question) {
	response.Question = []mDNS.Question{originalQuestion}
	for _, resourceRecord := range response.Answer {
		if strings.EqualFold(resourceRecord.Header().Name, expandedName) {
			resourceRecord.Header().Name = originalQuestion.Name
		}
	}
}
