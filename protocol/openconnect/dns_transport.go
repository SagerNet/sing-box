package openconnect

import (
	"context"
	"net/netip"
	"os"
	"strings"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/dns/transport"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	openconnecttransport "github.com/sagernet/sing-box/transport/openconnect"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"

	mDNS "github.com/miekg/dns"
)

func RegisterDNSTransport(registry *dns.TransportRegistry) {
	dns.RegisterTransport[option.OpenConnectDNSServerOptions](registry, C.DNSTypeOpenConnect, NewDNSTransport)
}

type DNSTransport struct {
	dns.TransportAdapter
	logger                 logger.ContextLogger
	endpointTag            string
	acceptDefaultResolvers bool
	acceptSearchDomain     bool
	endpointManager        adapter.EndpointManager
	endpoint               *Endpoint
	dialer                 N.Dialer
	access                 sync.RWMutex
	closed                 bool
	routes                 []openConnectDNSRoute
	searchDomains          []string
	defaultResolvers       []adapter.DNSTransport
}

type openConnectDNSRoute struct {
	domain    string
	resolvers []adapter.DNSTransport
}

func NewDNSTransport(ctx context.Context, logger log.ContextLogger, tag string, options option.OpenConnectDNSServerOptions) (adapter.DNSTransport, error) {
	if options.Endpoint == "" {
		return nil, E.New("missing endpoint tag")
	}
	return &DNSTransport{
		TransportAdapter:       dns.NewTransportAdapter(C.DNSTypeOpenConnect, tag, nil),
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
	openConnectEndpoint, isOpenConnect := rawEndpoint.(*Endpoint)
	if !isOpenConnect {
		return E.New("endpoint is not OpenConnect: ", t.endpointTag)
	}
	openConnectEndpoint.dnsTransportAccess.Lock()
	if openConnectEndpoint.dnsTransport != nil && openConnectEndpoint.dnsTransport.Tag() != t.Tag() {
		openConnectEndpoint.dnsTransportAccess.Unlock()
		return E.New("only one DNS server is allowed for an endpoint")
	}
	openConnectEndpoint.dnsTransport = t
	t.endpoint = openConnectEndpoint
	t.dialer = openConnectEndpoint
	state := openConnectEndpoint.state.Load()
	if state.started && state.tunnelConfigured && openConnectEndpoint.client.Ready() {
		t.updateConfiguration(state.configuration)
	}
	openConnectEndpoint.dnsTransportAccess.Unlock()
	return nil
}

func (t *DNSTransport) updateConfiguration(configuration openconnecttransport.Configuration) {
	resolverByAddress := make(map[netip.Addr]adapter.DNSTransport)
	resolverFor := func(address netip.Addr) adapter.DNSTransport {
		if !address.IsValid() {
			return nil
		}
		resolver, loaded := resolverByAddress[address]
		if loaded {
			return resolver
		}
		resolver = transport.NewUDPRaw(
			t.logger,
			dns.NewTransportAdapter(C.DNSTypeUDP, t.Tag()+"/"+address.String(), nil),
			t.dialer,
			M.SocksaddrFrom(address, 53),
		)
		resolverByAddress[address] = resolver
		return resolver
	}
	resolversFor := func(addresses []netip.Addr) []adapter.DNSTransport {
		resolvers := make([]adapter.DNSTransport, 0, len(addresses))
		resolverSet := make(map[adapter.DNSTransport]bool)
		for _, address := range addresses {
			resolver := resolverFor(address)
			if resolver != nil && !resolverSet[resolver] {
				resolverSet[resolver] = true
				resolvers = append(resolvers, resolver)
			}
		}
		return resolvers
	}
	defaultResolvers := resolversFor(configuration.DNS)
	routes := make([]openConnectDNSRoute, 0, len(configuration.SplitDNS)+len(configuration.SearchDomains)+len(configuration.SplitDNSRules))
	routeIndex := make(map[string]int)
	for _, rule := range configuration.SplitDNSRules {
		resolvers := resolversFor(rule.Servers)
		for _, domain := range rule.Domains {
			canonicalDomain := canonicalOpenConnectDomain(domain)
			if canonicalDomain != "" {
				fqdn := mDNS.Fqdn(canonicalDomain)
				index, loaded := routeIndex[fqdn]
				if loaded {
					resolverSet := make(map[adapter.DNSTransport]bool)
					for _, resolver := range routes[index].resolvers {
						resolverSet[resolver] = true
					}
					for _, resolver := range resolvers {
						if !resolverSet[resolver] {
							routes[index].resolvers = append(routes[index].resolvers, resolver)
						}
					}
				} else {
					routeIndex[fqdn] = len(routes)
					routes = append(routes, openConnectDNSRoute{domain: fqdn, resolvers: resolvers})
				}
			}
		}
	}
	for _, domain := range append(append([]string(nil), configuration.SplitDNS...), configuration.SearchDomains...) {
		canonicalDomain := canonicalOpenConnectDomain(domain)
		if canonicalDomain != "" {
			fqdn := mDNS.Fqdn(canonicalDomain)
			_, loaded := routeIndex[fqdn]
			if !loaded {
				routeIndex[fqdn] = len(routes)
				routes = append(routes, openConnectDNSRoute{domain: fqdn, resolvers: defaultResolvers})
			}
		}
	}
	searchDomains := make([]string, 0, len(configuration.SearchDomains))
	searchDomainSet := make(map[string]bool)
	for _, domain := range configuration.SearchDomains {
		canonicalDomain := canonicalOpenConnectDomain(domain)
		if canonicalDomain != "" {
			fqdn := mDNS.Fqdn(canonicalDomain)
			if !searchDomainSet[fqdn] {
				searchDomainSet[fqdn] = true
				searchDomains = append(searchDomains, fqdn)
			}
		}
	}
	if !t.acceptDefaultResolvers || !configuration.TunnelAllDNS && (len(configuration.SplitDNS) > 0 || len(configuration.SplitDNSRules) > 0) {
		defaultResolvers = nil
	}

	t.access.Lock()
	if t.closed {
		t.access.Unlock()
		for _, resolver := range resolverByAddress {
			_ = resolver.Close()
		}
		return
	}
	oldResolvers := t.collectResolversLocked()
	t.routes = routes
	t.searchDomains = searchDomains
	t.defaultResolvers = defaultResolvers
	activeResolvers := t.collectResolversLocked()
	t.access.Unlock()

	for _, resolver := range oldResolvers {
		_ = resolver.Close()
	}
	activeResolverSet := make(map[adapter.DNSTransport]bool, len(activeResolvers))
	for _, resolver := range activeResolvers {
		activeResolverSet[resolver] = true
	}
	for _, resolver := range resolverByAddress {
		if !activeResolverSet[resolver] {
			_ = resolver.Close()
		}
	}
	if len(resolverByAddress) > 0 {
		t.logger.Info("updated ", len(routes), " DNS routes and ", len(resolverByAddress), " resolvers")
	} else {
		t.logger.Info("cleared DNS configuration")
	}
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
		t.endpoint.dnsTransportAccess.Lock()
		if t.endpoint.dnsTransport == t {
			t.endpoint.dnsTransport = nil
		}
		t.endpoint.dnsTransportAccess.Unlock()
	}
	t.access.Lock()
	resolvers := t.collectResolversLocked()
	t.closed = true
	t.routes = nil
	t.searchDomains = nil
	t.defaultResolvers = nil
	t.access.Unlock()
	var closeErr error
	for _, resolver := range resolvers {
		closeErr = E.Errors(closeErr, resolver.Close())
	}
	return closeErr
}

func (t *DNSTransport) PreferredDomain(domain string) bool {
	canonicalDomain := mDNS.Fqdn(canonicalOpenConnectDomain(domain))
	t.access.RLock()
	routes := t.routes
	t.access.RUnlock()
	for _, route := range routes {
		if mDNS.IsSubDomain(route.domain, canonicalDomain) {
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
	searchDomains := append([]string(nil), t.searchDomains...)
	t.access.RUnlock()
	if t.acceptSearchDomain && len(searchDomains) > 0 && mDNS.CountLabel(message.Question[0].Name) == 1 {
		t.exchangeWithSearchDomains(ctx, message, searchDomains, callback)
		return
	}
	t.exchangeOnce(ctx, message, callback)
}

func (t *DNSTransport) exchangeWithSearchDomains(ctx context.Context, message *mDNS.Msg, searchDomains []string, callback func(response *mDNS.Msg, err error)) {
	originalQuestion := message.Question[0]
	singleLabel := strings.TrimSuffix(originalQuestion.Name, ".")
	exchangers := make([]transport.AsyncExchanger, 0, len(searchDomains)+1)
	for _, searchDomain := range searchDomains {
		expandedName := singleLabel + "." + searchDomain
		exchangers = append(exchangers, func(exchangeCtx context.Context, exchangeCallback func(response *mDNS.Msg, err error)) {
			question := originalQuestion
			question.Name = expandedName
			rewritten := *message
			rewritten.Question = []mDNS.Question{question}
			t.exchangeOnce(exchangeCtx, &rewritten, func(response *mDNS.Msg, err error) {
				if err == nil {
					restoreOpenConnectDNSQuestion(response, expandedName, originalQuestion)
				}
				exchangeCallback(response, err)
			})
		})
	}
	exchangers = append(exchangers, func(exchangeCtx context.Context, exchangeCallback func(response *mDNS.Msg, err error)) {
		t.exchangeOnce(exchangeCtx, message, exchangeCallback)
	})
	transport.ExchangeSequential(ctx, exchangers, func(response *mDNS.Msg, err error) bool {
		return err == nil && response.Rcode != mDNS.RcodeNameError
	}, callback)
}

func restoreOpenConnectDNSQuestion(response *mDNS.Msg, expandedName string, originalQuestion mDNS.Question) {
	response.Question = []mDNS.Question{originalQuestion}
	for _, record := range response.Answer {
		if strings.EqualFold(record.Header().Name, expandedName) {
			record.Header().Name = originalQuestion.Name
		}
	}
}

func (t *DNSTransport) exchangeOnce(ctx context.Context, message *mDNS.Msg, callback func(response *mDNS.Msg, err error)) {
	question := message.Question[0]
	t.access.RLock()
	routes := t.routes
	defaultResolvers := t.defaultResolvers
	t.access.RUnlock()
	var matchedResolvers []adapter.DNSTransport
	matchedDomainLength := -1
	for _, route := range routes {
		if len(route.domain) > matchedDomainLength && mDNS.IsSubDomain(route.domain, question.Name) {
			matchedDomainLength = len(route.domain)
			matchedResolvers = route.resolvers
		}
	}
	if matchedDomainLength != -1 {
		if len(matchedResolvers) == 0 {
			callback(nil, dns.RcodeNameError)
			return
		}
		transport.ExchangeSequential(ctx, openConnectDNSExchangers(matchedResolvers, message), nil, callback)
		return
	}
	if len(defaultResolvers) == 0 {
		callback(nil, dns.RcodeNameError)
		return
	}
	transport.ExchangeSequential(ctx, openConnectDNSExchangers(defaultResolvers, message), nil, callback)
}

func openConnectDNSExchangers(resolvers []adapter.DNSTransport, message *mDNS.Msg) []transport.AsyncExchanger {
	return common.Map(resolvers, func(resolver adapter.DNSTransport) transport.AsyncExchanger {
		return func(ctx context.Context, callback func(response *mDNS.Msg, err error)) {
			resolver.ExchangeAsync(ctx, message, callback)
		}
	})
}

func (t *DNSTransport) collectResolversLocked() []adapter.DNSTransport {
	resolverSet := make(map[adapter.DNSTransport]bool)
	for _, route := range t.routes {
		for _, resolver := range route.resolvers {
			resolverSet[resolver] = true
		}
	}
	for _, resolver := range t.defaultResolvers {
		resolverSet[resolver] = true
	}
	resolvers := make([]adapter.DNSTransport, 0, len(resolverSet))
	for resolver := range resolverSet {
		resolvers = append(resolvers, resolver)
	}
	return resolvers
}
