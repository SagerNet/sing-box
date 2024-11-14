package route

import (
	"context"
	"net/netip"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/geoip"
	"github.com/sagernet/sing-box/common/geosite"
	"github.com/sagernet/sing-box/common/process"
	"github.com/sagernet/sing-box/common/taskmonitor"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/libbox/platform"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	R "github.com/sagernet/sing-box/route/rule"
	"github.com/sagernet/sing-box/transport/fakeip"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/task"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/pause"
)

var _ adapter.Router = (*Router)(nil)

type Router struct {
	ctx                     context.Context
	logger                  log.ContextLogger
	dnsLogger               log.ContextLogger
	inboundManager          adapter.InboundManager
	outboundManager         adapter.OutboundManager
	networkManager          adapter.NetworkManager
	rules                   []adapter.Rule
	needGeoIPDatabase       bool
	needGeositeDatabase     bool
	geoIPOptions            option.GeoIPOptions
	geositeOptions          option.GeositeOptions
	geoIPReader             *geoip.Reader
	geositeReader           *geosite.Reader
	geositeCache            map[string]adapter.Rule
	needFindProcess         bool
	dnsClient               *dns.Client
	defaultDomainStrategy   dns.DomainStrategy
	dnsRules                []adapter.DNSRule
	ruleSets                []adapter.RuleSet
	ruleSetMap              map[string]adapter.RuleSet
	defaultTransport        dns.Transport
	transports              []dns.Transport
	transportMap            map[string]dns.Transport
	transportDomainStrategy map[dns.Transport]dns.DomainStrategy
	dnsReverseMapping       *DNSReverseMapping
	fakeIPStore             adapter.FakeIPStore
	processSearcher         process.Searcher
	pauseManager            pause.Manager
	tracker                 adapter.ConnectionTracker
	platformInterface       platform.Interface
	needWIFIState           bool
	started                 bool
}

func NewRouter(ctx context.Context, logFactory log.Factory, options option.RouteOptions, dnsOptions option.DNSOptions) (*Router, error) {
	router := &Router{
		ctx:                   ctx,
		logger:                logFactory.NewLogger("router"),
		dnsLogger:             logFactory.NewLogger("dns"),
		inboundManager:        service.FromContext[adapter.InboundManager](ctx),
		outboundManager:       service.FromContext[adapter.OutboundManager](ctx),
		networkManager:        service.FromContext[adapter.NetworkManager](ctx),
		rules:                 make([]adapter.Rule, 0, len(options.Rules)),
		dnsRules:              make([]adapter.DNSRule, 0, len(dnsOptions.Rules)),
		ruleSetMap:            make(map[string]adapter.RuleSet),
		needGeoIPDatabase:     hasRule(options.Rules, isGeoIPRule) || hasDNSRule(dnsOptions.Rules, isGeoIPDNSRule),
		needGeositeDatabase:   hasRule(options.Rules, isGeositeRule) || hasDNSRule(dnsOptions.Rules, isGeositeDNSRule),
		geoIPOptions:          common.PtrValueOrDefault(options.GeoIP),
		geositeOptions:        common.PtrValueOrDefault(options.Geosite),
		geositeCache:          make(map[string]adapter.Rule),
		needFindProcess:       hasRule(options.Rules, isProcessRule) || hasDNSRule(dnsOptions.Rules, isProcessDNSRule) || options.FindProcess,
		defaultDomainStrategy: dns.DomainStrategy(dnsOptions.Strategy),
		pauseManager:          service.FromContext[pause.Manager](ctx),
		platformInterface:     service.FromContext[platform.Interface](ctx),
		needWIFIState:         hasRule(options.Rules, isWIFIRule) || hasDNSRule(dnsOptions.Rules, isWIFIDNSRule),
	}
	service.MustRegister[adapter.Router](ctx, router)
	router.dnsClient = dns.NewClient(dns.ClientOptions{
		DisableCache:     dnsOptions.DNSClientOptions.DisableCache,
		DisableExpire:    dnsOptions.DNSClientOptions.DisableExpire,
		IndependentCache: dnsOptions.DNSClientOptions.IndependentCache,
		CacheCapacity:    dnsOptions.DNSClientOptions.CacheCapacity,
		RDRC: func() dns.RDRCStore {
			cacheFile := service.FromContext[adapter.CacheFile](ctx)
			if cacheFile == nil {
				return nil
			}
			if !cacheFile.StoreRDRC() {
				return nil
			}
			return cacheFile
		},
		Logger: router.dnsLogger,
	})
	for i, ruleOptions := range options.Rules {
		routeRule, err := R.NewRule(ctx, router.logger, ruleOptions, true)
		if err != nil {
			return nil, E.Cause(err, "parse rule[", i, "]")
		}
		router.rules = append(router.rules, routeRule)
	}
	for i, dnsRuleOptions := range dnsOptions.Rules {
		dnsRule, err := R.NewDNSRule(ctx, router.logger, dnsRuleOptions, true)
		if err != nil {
			return nil, E.Cause(err, "parse dns rule[", i, "]")
		}
		router.dnsRules = append(router.dnsRules, dnsRule)
	}
	for i, ruleSetOptions := range options.RuleSet {
		if _, exists := router.ruleSetMap[ruleSetOptions.Tag]; exists {
			return nil, E.New("duplicate rule-set tag: ", ruleSetOptions.Tag)
		}
		ruleSet, err := R.NewRuleSet(ctx, router.logger, ruleSetOptions)
		if err != nil {
			return nil, E.Cause(err, "parse rule-set[", i, "]")
		}
		router.ruleSets = append(router.ruleSets, ruleSet)
		router.ruleSetMap[ruleSetOptions.Tag] = ruleSet
	}

	transports := make([]dns.Transport, len(dnsOptions.Servers))
	dummyTransportMap := make(map[string]dns.Transport)
	transportMap := make(map[string]dns.Transport)
	transportTags := make([]string, len(dnsOptions.Servers))
	transportTagMap := make(map[string]bool)
	transportDomainStrategy := make(map[dns.Transport]dns.DomainStrategy)
	for i, server := range dnsOptions.Servers {
		var tag string
		if server.Tag != "" {
			tag = server.Tag
		} else {
			tag = F.ToString(i)
		}
		if transportTagMap[tag] {
			return nil, E.New("duplicate dns server tag: ", tag)
		}
		transportTags[i] = tag
		transportTagMap[tag] = true
	}
	outboundManager := service.FromContext[adapter.OutboundManager](ctx)
	for {
		lastLen := len(dummyTransportMap)
		for i, server := range dnsOptions.Servers {
			tag := transportTags[i]
			if _, exists := dummyTransportMap[tag]; exists {
				continue
			}
			var detour N.Dialer
			if server.Detour == "" {
				detour = dialer.NewDefaultOutbound(outboundManager)
			} else {
				detour = dialer.NewDetour(outboundManager, server.Detour)
			}
			var serverProtocol string
			switch server.Address {
			case "local":
				serverProtocol = "local"
			default:
				serverURL, _ := url.Parse(server.Address)
				var serverAddress string
				if serverURL != nil {
					if serverURL.Scheme == "" {
						serverProtocol = "udp"
					} else {
						serverProtocol = serverURL.Scheme
					}
					serverAddress = serverURL.Hostname()
				}
				if serverAddress == "" {
					serverAddress = server.Address
				}
				notIpAddress := !M.ParseSocksaddr(serverAddress).Addr.IsValid()
				if server.AddressResolver != "" {
					if !transportTagMap[server.AddressResolver] {
						return nil, E.New("parse dns server[", tag, "]: address resolver not found: ", server.AddressResolver)
					}
					if upstream, exists := dummyTransportMap[server.AddressResolver]; exists {
						detour = dns.NewDialerWrapper(detour, router.dnsClient, upstream, dns.DomainStrategy(server.AddressStrategy), time.Duration(server.AddressFallbackDelay))
					} else {
						continue
					}
				} else if notIpAddress && strings.Contains(server.Address, ".") {
					return nil, E.New("parse dns server[", tag, "]: missing address_resolver")
				}
			}
			var clientSubnet netip.Prefix
			if server.ClientSubnet != nil {
				clientSubnet = netip.Prefix(common.PtrValueOrDefault(server.ClientSubnet))
			} else if dnsOptions.ClientSubnet != nil {
				clientSubnet = netip.Prefix(common.PtrValueOrDefault(dnsOptions.ClientSubnet))
			}
			if serverProtocol == "" {
				serverProtocol = "transport"
			}
			transport, err := dns.CreateTransport(dns.TransportOptions{
				Context:      ctx,
				Logger:       logFactory.NewLogger(F.ToString("dns/", serverProtocol, "[", tag, "]")),
				Name:         tag,
				Dialer:       detour,
				Address:      server.Address,
				ClientSubnet: clientSubnet,
			})
			if err != nil {
				return nil, E.Cause(err, "parse dns server[", tag, "]")
			}
			transports[i] = transport
			dummyTransportMap[tag] = transport
			if server.Tag != "" {
				transportMap[server.Tag] = transport
			}
			strategy := dns.DomainStrategy(server.Strategy)
			if strategy != dns.DomainStrategyAsIS {
				transportDomainStrategy[transport] = strategy
			}
		}
		if len(transports) == len(dummyTransportMap) {
			break
		}
		if lastLen != len(dummyTransportMap) {
			continue
		}
		unresolvedTags := common.MapIndexed(common.FilterIndexed(dnsOptions.Servers, func(index int, server option.DNSServerOptions) bool {
			_, exists := dummyTransportMap[transportTags[index]]
			return !exists
		}), func(index int, server option.DNSServerOptions) string {
			return transportTags[index]
		})
		if len(unresolvedTags) == 0 {
			panic(F.ToString("unexpected unresolved dns servers: ", len(transports), " ", len(dummyTransportMap), " ", len(transportMap)))
		}
		return nil, E.New("found circular reference in dns servers: ", strings.Join(unresolvedTags, " "))
	}
	var defaultTransport dns.Transport
	if dnsOptions.Final != "" {
		defaultTransport = dummyTransportMap[dnsOptions.Final]
		if defaultTransport == nil {
			return nil, E.New("default dns server not found: ", dnsOptions.Final)
		}
	}
	if defaultTransport == nil {
		if len(transports) == 0 {
			transports = append(transports, common.Must1(dns.CreateTransport(dns.TransportOptions{
				Context: ctx,
				Name:    "local",
				Address: "local",
				Dialer:  common.Must1(dialer.NewDefault(router.networkManager, option.DialerOptions{})),
			})))
		}
		defaultTransport = transports[0]
	}
	if _, isFakeIP := defaultTransport.(adapter.FakeIPTransport); isFakeIP {
		return nil, E.New("default DNS server cannot be fakeip")
	}
	router.defaultTransport = defaultTransport
	router.transports = transports
	router.transportMap = transportMap
	router.transportDomainStrategy = transportDomainStrategy

	if dnsOptions.ReverseMapping {
		router.dnsReverseMapping = NewDNSReverseMapping()
	}

	if fakeIPOptions := dnsOptions.FakeIP; fakeIPOptions != nil && dnsOptions.FakeIP.Enabled {
		var inet4Range netip.Prefix
		var inet6Range netip.Prefix
		if fakeIPOptions.Inet4Range != nil {
			inet4Range = *fakeIPOptions.Inet4Range
		}
		if fakeIPOptions.Inet6Range != nil {
			inet6Range = *fakeIPOptions.Inet6Range
		}
		router.fakeIPStore = fakeip.NewStore(ctx, router.logger, inet4Range, inet6Range)
	}
	return router, nil
}

func (r *Router) Start(stage adapter.StartStage) error {
	monitor := taskmonitor.New(r.logger, C.StartTimeout)
	switch stage {
	case adapter.StartStateInitialize:
		if r.fakeIPStore != nil {
			monitor.Start("initialize fakeip store")
			err := r.fakeIPStore.Start()
			monitor.Finish()
			if err != nil {
				return err
			}
		}
	case adapter.StartStateStart:
		if r.needGeoIPDatabase {
			monitor.Start("initialize geoip database")
			err := r.prepareGeoIPDatabase()
			monitor.Finish()
			if err != nil {
				return err
			}
		}
		if r.needGeositeDatabase {
			monitor.Start("initialize geosite database")
			err := r.prepareGeositeDatabase()
			monitor.Finish()
			if err != nil {
				return err
			}
		}
		if r.needGeositeDatabase {
			for _, rule := range r.rules {
				err := rule.UpdateGeosite()
				if err != nil {
					r.logger.Error("failed to initialize geosite: ", err)
				}
			}
			for _, rule := range r.dnsRules {
				err := rule.UpdateGeosite()
				if err != nil {
					r.logger.Error("failed to initialize geosite: ", err)
				}
			}
			err := common.Close(r.geositeReader)
			if err != nil {
				return err
			}
			r.geositeCache = nil
			r.geositeReader = nil
		}

		monitor.Start("initialize DNS client")
		r.dnsClient.Start()
		monitor.Finish()

		for i, rule := range r.dnsRules {
			monitor.Start("initialize DNS rule[", i, "]")
			err := rule.Start()
			monitor.Finish()
			if err != nil {
				return E.Cause(err, "initialize DNS rule[", i, "]")
			}
		}
		for i, transport := range r.transports {
			monitor.Start("initialize DNS transport[", i, "]")
			err := transport.Start()
			monitor.Finish()
			if err != nil {
				return E.Cause(err, "initialize DNS server[", i, "]")
			}
		}
	case adapter.StartStatePostStart:
		var cacheContext *adapter.HTTPStartContext
		if len(r.ruleSets) > 0 {
			monitor.Start("initialize rule-set")
			cacheContext = adapter.NewHTTPStartContext()
			var ruleSetStartGroup task.Group
			for i, ruleSet := range r.ruleSets {
				ruleSetInPlace := ruleSet
				ruleSetStartGroup.Append0(func(ctx context.Context) error {
					err := ruleSetInPlace.StartContext(ctx, cacheContext)
					if err != nil {
						return E.Cause(err, "initialize rule-set[", i, "]")
					}
					return nil
				})
			}
			ruleSetStartGroup.Concurrency(5)
			ruleSetStartGroup.FastFail()
			err := ruleSetStartGroup.Run(r.ctx)
			monitor.Finish()
			if err != nil {
				return err
			}
		}
		if cacheContext != nil {
			cacheContext.Close()
		}
		needFindProcess := r.needFindProcess
		for _, ruleSet := range r.ruleSets {
			metadata := ruleSet.Metadata()
			if metadata.ContainsProcessRule {
				needFindProcess = true
			}
			if metadata.ContainsWIFIRule {
				r.needWIFIState = true
			}
		}
		if needFindProcess {
			if r.platformInterface != nil {
				r.processSearcher = r.platformInterface
			} else {
				monitor.Start("initialize process searcher")
				searcher, err := process.NewSearcher(process.Config{
					Logger:         r.logger,
					PackageManager: r.networkManager.PackageManager(),
				})
				monitor.Finish()
				if err != nil {
					if err != os.ErrInvalid {
						r.logger.Warn(E.Cause(err, "create process searcher"))
					}
				} else {
					r.processSearcher = searcher
				}
			}
		}
		for i, rule := range r.rules {
			monitor.Start("initialize rule[", i, "]")
			err := rule.Start()
			monitor.Finish()
			if err != nil {
				return E.Cause(err, "initialize rule[", i, "]")
			}
		}
		for _, ruleSet := range r.ruleSets {
			monitor.Start("post start rule_set[", ruleSet.Name(), "]")
			err := ruleSet.PostStart()
			monitor.Finish()
			if err != nil {
				return E.Cause(err, "post start rule_set[", ruleSet.Name(), "]")
			}
		}
		r.started = true
		return nil
	case adapter.StartStateStarted:
		for _, ruleSet := range r.ruleSetMap {
			ruleSet.Cleanup()
		}
		runtime.GC()
	}
	return nil
}

func (r *Router) Close() error {
	monitor := taskmonitor.New(r.logger, C.StopTimeout)
	var err error
	for i, rule := range r.rules {
		monitor.Start("close rule[", i, "]")
		err = E.Append(err, rule.Close(), func(err error) error {
			return E.Cause(err, "close rule[", i, "]")
		})
		monitor.Finish()
	}
	for i, rule := range r.dnsRules {
		monitor.Start("close dns rule[", i, "]")
		err = E.Append(err, rule.Close(), func(err error) error {
			return E.Cause(err, "close dns rule[", i, "]")
		})
		monitor.Finish()
	}
	for i, transport := range r.transports {
		monitor.Start("close dns transport[", i, "]")
		err = E.Append(err, transport.Close(), func(err error) error {
			return E.Cause(err, "close dns transport[", i, "]")
		})
		monitor.Finish()
	}
	if r.geoIPReader != nil {
		monitor.Start("close geoip reader")
		err = E.Append(err, r.geoIPReader.Close(), func(err error) error {
			return E.Cause(err, "close geoip reader")
		})
		monitor.Finish()
	}
	if r.fakeIPStore != nil {
		monitor.Start("close fakeip store")
		err = E.Append(err, r.fakeIPStore.Close(), func(err error) error {
			return E.Cause(err, "close fakeip store")
		})
		monitor.Finish()
	}
	return err
}

func (r *Router) FakeIPStore() adapter.FakeIPStore {
	return r.fakeIPStore
}

func (r *Router) RuleSet(tag string) (adapter.RuleSet, bool) {
	ruleSet, loaded := r.ruleSetMap[tag]
	return ruleSet, loaded
}

func (r *Router) NeedWIFIState() bool {
	return r.needWIFIState
}

func (r *Router) Rules() []adapter.Rule {
	return r.rules
}

func (r *Router) SetTracker(tracker adapter.ConnectionTracker) {
	r.tracker = tracker
}

func (r *Router) ResetNetwork() {
	r.networkManager.ResetNetwork()
	for _, transport := range r.transports {
		transport.Reset()
	}
}
