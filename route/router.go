package route

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/geoip"
	"github.com/sagernet/sing-box/common/geosite"
	"github.com/sagernet/sing-box/common/mux"
	"github.com/sagernet/sing-box/common/process"
	"github.com/sagernet/sing-box/common/sniff"
	"github.com/sagernet/sing-box/common/warning"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/libbox/platform"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/ntp"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/outbound"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing-vmess"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/rw"
	"github.com/sagernet/sing/common/uot"
)

var warnDefaultInterfaceOnUnsupportedPlatform = warning.New(
	func() bool {
		return !(C.IsLinux || C.IsWindows || C.IsDarwin)
	},
	"route option `default_mark` is only supported on Linux and Windows",
)

var warnDefaultMarkOnNonLinux = warning.New(
	func() bool {
		return !C.IsLinux
	},
	"route option `default_mark` is only supported on Linux",
)

var warnFindProcessOnUnsupportedPlatform = warning.New(
	func() bool {
		return !(C.IsLinux || C.IsWindows || C.IsDarwin)
	},
	"route option `find_process` is only supported on Linux, Windows, and macOS",
)

var _ adapter.Router = (*Router)(nil)

type Router struct {
	ctx                                context.Context
	logger                             log.ContextLogger
	dnsLogger                          log.ContextLogger
	inboundByTag                       map[string]adapter.Inbound
	outbounds                          []adapter.Outbound
	outboundByTag                      map[string]adapter.Outbound
	rules                              []adapter.Rule
	defaultDetour                      string
	defaultOutboundForConnection       adapter.Outbound
	defaultOutboundForPacketConnection adapter.Outbound
	needGeoIPDatabase                  bool
	needGeositeDatabase                bool
	geoIPOptions                       option.GeoIPOptions
	geositeOptions                     option.GeositeOptions
	geoIPReader                        *geoip.Reader
	geositeReader                      *geosite.Reader
	geositeCache                       map[string]adapter.Rule
	dnsClient                          *dns.Client
	defaultDomainStrategy              dns.DomainStrategy
	dnsRules                           []adapter.DNSRule
	defaultTransport                   dns.Transport
	transports                         []dns.Transport
	transportMap                       map[string]dns.Transport
	transportDomainStrategy            map[dns.Transport]dns.DomainStrategy
	dnsReverseMapping                  *DNSReverseMapping
	interfaceFinder                    myInterfaceFinder
	autoDetectInterface                bool
	defaultInterface                   string
	defaultMark                        int
	networkMonitor                     tun.NetworkUpdateMonitor
	interfaceMonitor                   tun.DefaultInterfaceMonitor
	packageManager                     tun.PackageManager
	processSearcher                    process.Searcher
	timeService                        adapter.TimeService
	clashServer                        adapter.ClashServer
	v2rayServer                        adapter.V2RayServer
	platformInterface                  platform.Interface
}

func NewRouter(
	ctx context.Context,
	logFactory log.Factory,
	options option.RouteOptions,
	dnsOptions option.DNSOptions,
	ntpOptions option.NTPOptions,
	inbounds []option.Inbound,
	platformInterface platform.Interface,
) (*Router, error) {
	if options.DefaultInterface != "" {
		warnDefaultInterfaceOnUnsupportedPlatform.Check()
	}
	if options.DefaultMark != 0 {
		warnDefaultMarkOnNonLinux.Check()
	}
	if options.FindProcess {
		warnFindProcessOnUnsupportedPlatform.Check()
	}

	router := &Router{
		ctx:                   ctx,
		logger:                logFactory.NewLogger("router"),
		dnsLogger:             logFactory.NewLogger("dns"),
		outboundByTag:         make(map[string]adapter.Outbound),
		rules:                 make([]adapter.Rule, 0, len(options.Rules)),
		dnsRules:              make([]adapter.DNSRule, 0, len(dnsOptions.Rules)),
		needGeoIPDatabase:     hasRule(options.Rules, isGeoIPRule) || hasDNSRule(dnsOptions.Rules, isGeoIPDNSRule),
		needGeositeDatabase:   hasRule(options.Rules, isGeositeRule) || hasDNSRule(dnsOptions.Rules, isGeositeDNSRule),
		geoIPOptions:          common.PtrValueOrDefault(options.GeoIP),
		geositeOptions:        common.PtrValueOrDefault(options.Geosite),
		geositeCache:          make(map[string]adapter.Rule),
		defaultDetour:         options.Final,
		defaultDomainStrategy: dns.DomainStrategy(dnsOptions.Strategy),
		autoDetectInterface:   options.AutoDetectInterface,
		defaultInterface:      options.DefaultInterface,
		defaultMark:           options.DefaultMark,
		platformInterface:     platformInterface,
	}
	router.dnsClient = dns.NewClient(dnsOptions.DNSClientOptions.DisableCache, dnsOptions.DNSClientOptions.DisableExpire, router.dnsLogger)
	for i, ruleOptions := range options.Rules {
		routeRule, err := NewRule(router, router.logger, ruleOptions)
		if err != nil {
			return nil, E.Cause(err, "parse rule[", i, "]")
		}
		router.rules = append(router.rules, routeRule)
	}
	for i, dnsRuleOptions := range dnsOptions.Rules {
		dnsRule, err := NewDNSRule(router, router.logger, dnsRuleOptions)
		if err != nil {
			return nil, E.Cause(err, "parse dns rule[", i, "]")
		}
		router.dnsRules = append(router.dnsRules, dnsRule)
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
	ctx = adapter.ContextWithRouter(ctx, router)
	for {
		lastLen := len(dummyTransportMap)
		for i, server := range dnsOptions.Servers {
			tag := transportTags[i]
			if _, exists := dummyTransportMap[tag]; exists {
				continue
			}
			var detour N.Dialer
			if server.Detour == "" {
				detour = dialer.NewRouter(router)
			} else {
				detour = dialer.NewDetour(router, server.Detour)
			}
			switch server.Address {
			case "local":
			default:
				serverURL, _ := url.Parse(server.Address)
				var serverAddress string
				if serverURL != nil {
					serverAddress = serverURL.Hostname()
				}
				if serverAddress == "" {
					serverAddress = server.Address
				}
				_, notIpAddress := netip.ParseAddr(serverAddress)
				if server.AddressResolver != "" {
					if !transportTagMap[server.AddressResolver] {
						return nil, E.New("parse dns server[", tag, "]: address resolver not found: ", server.AddressResolver)
					}
					if upstream, exists := dummyTransportMap[server.AddressResolver]; exists {
						detour = dns.NewDialerWrapper(detour, router.dnsClient, upstream, dns.DomainStrategy(server.AddressStrategy), time.Duration(server.AddressFallbackDelay))
					} else {
						continue
					}
				} else if notIpAddress != nil {
					switch serverURL.Scheme {
					case "rcode", "dhcp":
					default:
						return nil, E.New("parse dns server[", tag, "]: missing address_resolver")
					}
				}
			}
			transport, err := dns.CreateTransport(tag, ctx, logFactory.NewLogger(F.ToString("dns/transport[", tag, "]")), detour, server.Address)
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
			transports = append(transports, dns.NewLocalTransport("local", N.SystemDialer))
		}
		defaultTransport = transports[0]
	}
	router.defaultTransport = defaultTransport
	router.transports = transports
	router.transportMap = transportMap
	router.transportDomainStrategy = transportDomainStrategy

	if dnsOptions.ReverseMapping {
		router.dnsReverseMapping = NewDNSReverseMapping()
	}

	needInterfaceMonitor := platformInterface == nil && (options.AutoDetectInterface || common.Any(inbounds, func(inbound option.Inbound) bool {
		return inbound.HTTPOptions.SetSystemProxy || inbound.MixedOptions.SetSystemProxy || inbound.TunOptions.AutoRoute
	}))

	if needInterfaceMonitor {
		networkMonitor, err := tun.NewNetworkUpdateMonitor(router)
		if err == nil {
			router.networkMonitor = networkMonitor
			networkMonitor.RegisterCallback(router.interfaceFinder.update)
		}
	}

	if router.networkMonitor != nil && needInterfaceMonitor {
		interfaceMonitor, err := tun.NewDefaultInterfaceMonitor(router.networkMonitor, tun.DefaultInterfaceMonitorOptions{
			OverrideAndroidVPN: options.OverrideAndroidVPN,
		})
		if err != nil {
			return nil, E.New("auto_detect_interface unsupported on current platform")
		}
		interfaceMonitor.RegisterCallback(router.notifyNetworkUpdate)
		router.interfaceMonitor = interfaceMonitor
	}

	needFindProcess := hasRule(options.Rules, isProcessRule) || hasDNSRule(dnsOptions.Rules, isProcessDNSRule) || options.FindProcess
	needPackageManager := C.IsAndroid && platformInterface == nil && (needFindProcess || common.Any(inbounds, func(inbound option.Inbound) bool {
		return len(inbound.TunOptions.IncludePackage) > 0 || len(inbound.TunOptions.ExcludePackage) > 0
	}))
	if needPackageManager {
		packageManager, err := tun.NewPackageManager(router)
		if err != nil {
			return nil, E.Cause(err, "create package manager")
		}
		router.packageManager = packageManager
	}
	if needFindProcess {
		if platformInterface != nil {
			router.processSearcher = platformInterface
		} else {
			searcher, err := process.NewSearcher(process.Config{
				Logger:         logFactory.NewLogger("router/process"),
				PackageManager: router.packageManager,
			})
			if err != nil {
				if err != os.ErrInvalid {
					router.logger.Warn(E.Cause(err, "create process searcher"))
				}
			} else {
				router.processSearcher = searcher
			}
		}
	}
	if ntpOptions.Enabled {
		router.timeService = ntp.NewService(ctx, router, logFactory.NewLogger("ntp"), ntpOptions)
	}
	return router, nil
}

func (r *Router) Initialize(inbounds []adapter.Inbound, outbounds []adapter.Outbound, defaultOutbound func() adapter.Outbound) error {
	inboundByTag := make(map[string]adapter.Inbound)
	for _, inbound := range inbounds {
		inboundByTag[inbound.Tag()] = inbound
	}
	outboundByTag := make(map[string]adapter.Outbound)
	for _, detour := range outbounds {
		outboundByTag[detour.Tag()] = detour
	}
	var defaultOutboundForConnection adapter.Outbound
	var defaultOutboundForPacketConnection adapter.Outbound
	if r.defaultDetour != "" {
		detour, loaded := outboundByTag[r.defaultDetour]
		if !loaded {
			return E.New("default detour not found: ", r.defaultDetour)
		}
		if common.Contains(detour.Network(), N.NetworkTCP) {
			defaultOutboundForConnection = detour
		}
		if common.Contains(detour.Network(), N.NetworkUDP) {
			defaultOutboundForPacketConnection = detour
		}
	}
	var index, packetIndex int
	if defaultOutboundForConnection == nil {
		for i, detour := range outbounds {
			if common.Contains(detour.Network(), N.NetworkTCP) {
				index = i
				defaultOutboundForConnection = detour
				break
			}
		}
	}
	if defaultOutboundForPacketConnection == nil {
		for i, detour := range outbounds {
			if common.Contains(detour.Network(), N.NetworkUDP) {
				packetIndex = i
				defaultOutboundForPacketConnection = detour
				break
			}
		}
	}
	if defaultOutboundForConnection == nil || defaultOutboundForPacketConnection == nil {
		detour := defaultOutbound()
		if defaultOutboundForConnection == nil {
			defaultOutboundForConnection = detour
		}
		if defaultOutboundForPacketConnection == nil {
			defaultOutboundForPacketConnection = detour
		}
		outbounds = append(outbounds, detour)
		outboundByTag[detour.Tag()] = detour
	}
	if defaultOutboundForConnection != defaultOutboundForPacketConnection {
		var description string
		if defaultOutboundForConnection.Tag() != "" {
			description = defaultOutboundForConnection.Tag()
		} else {
			description = F.ToString(index)
		}
		var packetDescription string
		if defaultOutboundForPacketConnection.Tag() != "" {
			packetDescription = defaultOutboundForPacketConnection.Tag()
		} else {
			packetDescription = F.ToString(packetIndex)
		}
		r.logger.Info("using ", defaultOutboundForConnection.Type(), "[", description, "] as default outbound for connection")
		r.logger.Info("using ", defaultOutboundForPacketConnection.Type(), "[", packetDescription, "] as default outbound for packet connection")
	}
	r.inboundByTag = inboundByTag
	r.outbounds = outbounds
	r.defaultOutboundForConnection = defaultOutboundForConnection
	r.defaultOutboundForPacketConnection = defaultOutboundForPacketConnection
	r.outboundByTag = outboundByTag
	for i, rule := range r.rules {
		if _, loaded := outboundByTag[rule.Outbound()]; !loaded {
			return E.New("outbound not found for rule[", i, "]: ", rule.Outbound())
		}
	}
	return nil
}

func (r *Router) Outbounds() []adapter.Outbound {
	return r.outbounds
}

func (r *Router) Start() error {
	if r.needGeoIPDatabase {
		err := r.prepareGeoIPDatabase()
		if err != nil {
			return err
		}
	}
	if r.needGeositeDatabase {
		err := r.prepareGeositeDatabase()
		if err != nil {
			return err
		}
	}
	if r.interfaceMonitor != nil {
		err := r.interfaceMonitor.Start()
		if err != nil {
			return err
		}
	}
	if r.networkMonitor != nil {
		err := r.networkMonitor.Start()
		if err != nil {
			return err
		}
	}
	if r.packageManager != nil {
		err := r.packageManager.Start()
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
	for i, rule := range r.rules {
		err := rule.Start()
		if err != nil {
			return E.Cause(err, "initialize rule[", i, "]")
		}
	}
	for i, rule := range r.dnsRules {
		err := rule.Start()
		if err != nil {
			return E.Cause(err, "initialize DNS rule[", i, "]")
		}
	}
	for i, transport := range r.transports {
		err := transport.Start()
		if err != nil {
			return E.Cause(err, "initialize DNS server[", i, "]")
		}
	}
	if r.timeService != nil {
		err := r.timeService.Start()
		if err != nil {
			return E.Cause(err, "initialize time service")
		}
	}
	return nil
}

func (r *Router) Close() error {
	var err error
	for i, rule := range r.rules {
		r.logger.Trace("closing rule[", i, "]")
		err = E.Append(err, rule.Close(), func(err error) error {
			return E.Cause(err, "close rule[", i, "]")
		})
	}
	for i, rule := range r.dnsRules {
		r.logger.Trace("closing dns rule[", i, "]")
		err = E.Append(err, rule.Close(), func(err error) error {
			return E.Cause(err, "close dns rule[", i, "]")
		})
	}
	for i, transport := range r.transports {
		r.logger.Trace("closing transport[", i, "] ")
		err = E.Append(err, transport.Close(), func(err error) error {
			return E.Cause(err, "close dns transport[", i, "]")
		})
	}
	if r.geositeReader != nil {
		r.logger.Trace("closing geoip reader")
		err = E.Append(err, common.Close(r.geoIPReader), func(err error) error {
			return E.Cause(err, "close geoip reader")
		})
	}
	if r.interfaceMonitor != nil {
		r.logger.Trace("closing interface monitor")
		err = E.Append(err, r.interfaceMonitor.Close(), func(err error) error {
			return E.Cause(err, "close interface monitor")
		})
	}
	if r.networkMonitor != nil {
		r.logger.Trace("closing network monitor")
		err = E.Append(err, r.networkMonitor.Close(), func(err error) error {
			return E.Cause(err, "close network monitor")
		})
	}
	if r.packageManager != nil {
		r.logger.Trace("closing package manager")
		err = E.Append(err, r.packageManager.Close(), func(err error) error {
			return E.Cause(err, "close package manager")
		})
	}
	if r.timeService != nil {
		r.logger.Trace("closing time service")
		err = E.Append(err, r.timeService.Close(), func(err error) error {
			return E.Cause(err, "close time service")
		})
	}
	return err
}

func (r *Router) GeoIPReader() *geoip.Reader {
	return r.geoIPReader
}

func (r *Router) LoadGeosite(code string) (adapter.Rule, error) {
	rule, cached := r.geositeCache[code]
	if cached {
		return rule, nil
	}
	items, err := r.geositeReader.Read(code)
	if err != nil {
		return nil, err
	}
	rule, err = NewDefaultRule(r, nil, geosite.Compile(items))
	if err != nil {
		return nil, err
	}
	r.geositeCache[code] = rule
	return rule, nil
}

func (r *Router) Outbound(tag string) (adapter.Outbound, bool) {
	outbound, loaded := r.outboundByTag[tag]
	return outbound, loaded
}

func (r *Router) DefaultOutbound(network string) adapter.Outbound {
	if network == N.NetworkTCP {
		return r.defaultOutboundForConnection
	} else {
		return r.defaultOutboundForPacketConnection
	}
}

func (r *Router) RouteConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	if metadata.InboundDetour != "" {
		if metadata.LastInbound == metadata.InboundDetour {
			return E.New("routing loop on detour: ", metadata.InboundDetour)
		}
		detour := r.inboundByTag[metadata.InboundDetour]
		if detour == nil {
			return E.New("inbound detour not found: ", metadata.InboundDetour)
		}
		injectable, isInjectable := detour.(adapter.InjectableInbound)
		if !isInjectable {
			return E.New("inbound detour is not injectable: ", metadata.InboundDetour)
		}
		if !common.Contains(injectable.Network(), N.NetworkTCP) {
			return E.New("inject: TCP unsupported")
		}
		metadata.LastInbound = metadata.Inbound
		metadata.Inbound = metadata.InboundDetour
		metadata.InboundDetour = ""
		err := injectable.NewConnection(ctx, conn, metadata)
		if err != nil {
			return E.Cause(err, "inject ", detour.Tag())
		}
		return nil
	}
	metadata.Network = N.NetworkTCP
	switch metadata.Destination.Fqdn {
	case mux.Destination.Fqdn:
		r.logger.InfoContext(ctx, "inbound multiplex connection")
		return mux.NewConnection(ctx, r, r, r.logger, conn, metadata)
	case vmess.MuxDestination.Fqdn:
		r.logger.InfoContext(ctx, "inbound legacy multiplex connection")
		return vmess.HandleMuxConnection(ctx, conn, adapter.NewUpstreamHandler(metadata, r.RouteConnection, r.RoutePacketConnection, r))
	case uot.MagicAddress:
		request, err := uot.ReadRequest(conn)
		if err != nil {
			return E.Cause(err, "read UoT request")
		}
		if request.IsConnect {
			r.logger.InfoContext(ctx, "inbound UoT connect connection to ", request.Destination)
		} else {
			r.logger.InfoContext(ctx, "inbound UoT connection to ", request.Destination)
		}
		metadata.Domain = metadata.Destination.Fqdn
		metadata.Destination = request.Destination
		return r.RoutePacketConnection(ctx, uot.NewConn(conn, *request), metadata)
	case uot.LegacyMagicAddress:
		r.logger.InfoContext(ctx, "inbound legacy UoT connection")
		metadata.Domain = metadata.Destination.Fqdn
		metadata.Destination = M.Socksaddr{Addr: netip.IPv4Unspecified()}
		return r.RoutePacketConnection(ctx, uot.NewConn(conn, uot.Request{}), metadata)
	}
	if metadata.InboundOptions.SniffEnabled {
		buffer := buf.NewPacket()
		buffer.FullReset()
		sniffMetadata, err := sniff.PeekStream(ctx, conn, buffer, time.Duration(metadata.InboundOptions.SniffTimeout), sniff.StreamDomainNameQuery, sniff.TLSClientHello, sniff.HTTPHost)
		if sniffMetadata != nil {
			metadata.Protocol = sniffMetadata.Protocol
			metadata.Domain = sniffMetadata.Domain
			if metadata.InboundOptions.SniffOverrideDestination && M.IsDomainName(metadata.Domain) {
				metadata.Destination = M.Socksaddr{
					Fqdn: metadata.Domain,
					Port: metadata.Destination.Port,
				}
			}
			if metadata.Domain != "" {
				r.logger.DebugContext(ctx, "sniffed protocol: ", metadata.Protocol, ", domain: ", metadata.Domain)
			} else {
				r.logger.DebugContext(ctx, "sniffed protocol: ", metadata.Protocol)
			}
		} else if err != nil {
			r.logger.TraceContext(ctx, "sniffed no protocol: ", err)
		}
		if !buffer.IsEmpty() {
			conn = bufio.NewCachedConn(conn, buffer)
		} else {
			buffer.Release()
		}
	}

	if r.dnsReverseMapping != nil && metadata.Domain == "" {
		domain, loaded := r.dnsReverseMapping.Query(metadata.Destination.Addr)
		if loaded {
			metadata.Domain = domain
			r.logger.DebugContext(ctx, "found reserve mapped domain: ", metadata.Domain)
		}
	}

	if metadata.Destination.IsFqdn() && dns.DomainStrategy(metadata.InboundOptions.DomainStrategy) != dns.DomainStrategyAsIS {
		addresses, err := r.Lookup(adapter.WithContext(ctx, &metadata), metadata.Destination.Fqdn, dns.DomainStrategy(metadata.InboundOptions.DomainStrategy))
		if err != nil {
			return err
		}
		metadata.DestinationAddresses = addresses
		r.dnsLogger.DebugContext(ctx, "resolved [", strings.Join(F.MapToString(metadata.DestinationAddresses), " "), "]")
	}
	ctx, matchedRule, detour, err := r.match(ctx, &metadata, r.defaultOutboundForConnection)
	if err != nil {
		return err
	}
	if !common.Contains(detour.Network(), N.NetworkTCP) {
		return E.New("missing supported outbound, closing connection")
	}
	if r.clashServer != nil {
		trackerConn, tracker := r.clashServer.RoutedConnection(ctx, conn, metadata, matchedRule)
		defer tracker.Leave()
		conn = trackerConn
	}
	if r.v2rayServer != nil {
		if statsService := r.v2rayServer.StatsService(); statsService != nil {
			conn = statsService.RoutedConnection(metadata.Inbound, detour.Tag(), metadata.User, conn)
		}
	}
	return detour.NewConnection(ctx, conn, metadata)
}

func (r *Router) RoutePacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	if metadata.InboundDetour != "" {
		if metadata.LastInbound == metadata.InboundDetour {
			return E.New("routing loop on detour: ", metadata.InboundDetour)
		}
		detour := r.inboundByTag[metadata.InboundDetour]
		if detour == nil {
			return E.New("inbound detour not found: ", metadata.InboundDetour)
		}
		injectable, isInjectable := detour.(adapter.InjectableInbound)
		if !isInjectable {
			return E.New("inbound detour is not injectable: ", metadata.InboundDetour)
		}
		if !common.Contains(injectable.Network(), N.NetworkUDP) {
			return E.New("inject: UDP unsupported")
		}
		metadata.LastInbound = metadata.Inbound
		metadata.Inbound = metadata.InboundDetour
		metadata.InboundDetour = ""
		err := injectable.NewPacketConnection(ctx, conn, metadata)
		if err != nil {
			return E.Cause(err, "inject ", detour.Tag())
		}
		return nil
	}
	metadata.Network = N.NetworkUDP
	if metadata.InboundOptions.SniffEnabled {
		buffer := buf.NewPacket()
		buffer.FullReset()
		destination, err := conn.ReadPacket(buffer)
		if err != nil {
			buffer.Release()
			return err
		}
		sniffMetadata, _ := sniff.PeekPacket(ctx, buffer.Bytes(), sniff.DomainNameQuery, sniff.QUICClientHello, sniff.STUNMessage)
		if sniffMetadata != nil {
			metadata.Protocol = sniffMetadata.Protocol
			metadata.Domain = sniffMetadata.Domain
			if metadata.InboundOptions.SniffOverrideDestination && M.IsDomainName(metadata.Domain) {
				metadata.Destination = M.Socksaddr{
					Fqdn: metadata.Domain,
					Port: metadata.Destination.Port,
				}
			}
			if metadata.Domain != "" {
				r.logger.DebugContext(ctx, "sniffed packet protocol: ", metadata.Protocol, ", domain: ", metadata.Domain)
			} else {
				r.logger.DebugContext(ctx, "sniffed packet protocol: ", metadata.Protocol)
			}
		}
		conn = bufio.NewCachedPacketConn(conn, buffer, destination)
	}
	if metadata.Destination.IsFqdn() && dns.DomainStrategy(metadata.InboundOptions.DomainStrategy) != dns.DomainStrategyAsIS {
		addresses, err := r.Lookup(adapter.WithContext(ctx, &metadata), metadata.Destination.Fqdn, dns.DomainStrategy(metadata.InboundOptions.DomainStrategy))
		if err != nil {
			return err
		}
		metadata.DestinationAddresses = addresses
		r.dnsLogger.DebugContext(ctx, "resolved [", strings.Join(F.MapToString(metadata.DestinationAddresses), " "), "]")
	}
	ctx, matchedRule, detour, err := r.match(ctx, &metadata, r.defaultOutboundForPacketConnection)
	if err != nil {
		return err
	}
	if !common.Contains(detour.Network(), N.NetworkUDP) {
		return E.New("missing supported outbound, closing packet connection")
	}
	if r.clashServer != nil {
		trackerConn, tracker := r.clashServer.RoutedPacketConnection(ctx, conn, metadata, matchedRule)
		defer tracker.Leave()
		conn = trackerConn
	}
	if r.v2rayServer != nil {
		if statsService := r.v2rayServer.StatsService(); statsService != nil {
			conn = statsService.RoutedPacketConnection(metadata.Inbound, detour.Tag(), metadata.User, conn)
		}
	}
	return detour.NewPacketConnection(ctx, conn, metadata)
}

func (r *Router) match(ctx context.Context, metadata *adapter.InboundContext, defaultOutbound adapter.Outbound) (context.Context, adapter.Rule, adapter.Outbound, error) {
	matchRule, matchOutbound := r.match0(ctx, metadata, defaultOutbound)
	if contextOutbound, loaded := outbound.TagFromContext(ctx); loaded {
		if contextOutbound == matchOutbound.Tag() {
			return nil, nil, nil, E.New("connection loopback in outbound/", matchOutbound.Type(), "[", matchOutbound.Tag(), "]")
		}
	}
	ctx = outbound.ContextWithTag(ctx, matchOutbound.Tag())
	return ctx, matchRule, matchOutbound, nil
}

func (r *Router) match0(ctx context.Context, metadata *adapter.InboundContext, defaultOutbound adapter.Outbound) (adapter.Rule, adapter.Outbound) {
	if r.processSearcher != nil {
		var originDestination netip.AddrPort
		if metadata.OriginDestination.IsValid() {
			originDestination = metadata.OriginDestination.AddrPort()
		} else if metadata.Destination.IsIP() {
			originDestination = metadata.Destination.AddrPort()
		}
		processInfo, err := process.FindProcessInfo(r.processSearcher, ctx, metadata.Network, metadata.Source.AddrPort(), originDestination)
		if err != nil {
			r.logger.InfoContext(ctx, "failed to search process: ", err)
		} else {
			if processInfo.ProcessPath != "" {
				r.logger.InfoContext(ctx, "found process path: ", processInfo.ProcessPath)
			} else if processInfo.PackageName != "" {
				r.logger.InfoContext(ctx, "found package name: ", processInfo.PackageName)
			} else if processInfo.UserId != -1 {
				if /*needUserName &&*/ true {
					osUser, _ := user.LookupId(F.ToString(processInfo.UserId))
					if osUser != nil {
						processInfo.User = osUser.Username
					}
				}
				if processInfo.User != "" {
					r.logger.InfoContext(ctx, "found user: ", processInfo.User)
				} else {
					r.logger.InfoContext(ctx, "found user id: ", processInfo.UserId)
				}
			}
			metadata.ProcessInfo = processInfo
		}
	}
	for i, rule := range r.rules {
		if rule.Match(metadata) {
			detour := rule.Outbound()
			r.logger.DebugContext(ctx, "match[", i, "] ", rule.String(), " => ", detour)
			if outbound, loaded := r.Outbound(detour); loaded {
				return rule, outbound
			}
			r.logger.ErrorContext(ctx, "outbound not found: ", detour)
		}
	}
	return nil, defaultOutbound
}

func (r *Router) InterfaceFinder() control.InterfaceFinder {
	return &r.interfaceFinder
}

func (r *Router) AutoDetectInterface() bool {
	return r.autoDetectInterface
}

func (r *Router) AutoDetectInterfaceFunc() control.Func {
	if r.platformInterface != nil {
		return r.platformInterface.AutoDetectInterfaceControl()
	} else {
		return control.BindToInterfaceFunc(r.InterfaceFinder(), func(network string, address string) (interfaceName string, interfaceIndex int) {
			remoteAddr := M.ParseSocksaddr(address).Addr
			if C.IsLinux {
				return r.InterfaceMonitor().DefaultInterfaceName(remoteAddr), -1
			} else {
				return "", r.InterfaceMonitor().DefaultInterfaceIndex(remoteAddr)
			}
		})
	}
}

func (r *Router) DefaultInterface() string {
	return r.defaultInterface
}

func (r *Router) DefaultMark() int {
	return r.defaultMark
}

func (r *Router) Rules() []adapter.Rule {
	return r.rules
}

func (r *Router) NetworkMonitor() tun.NetworkUpdateMonitor {
	return r.networkMonitor
}

func (r *Router) InterfaceMonitor() tun.DefaultInterfaceMonitor {
	return r.interfaceMonitor
}

func (r *Router) PackageManager() tun.PackageManager {
	return r.packageManager
}

func (r *Router) TimeFunc() func() time.Time {
	if r.timeService == nil {
		return nil
	}
	return r.timeService.TimeFunc()
}

func (r *Router) ClashServer() adapter.ClashServer {
	return r.clashServer
}

func (r *Router) SetClashServer(server adapter.ClashServer) {
	r.clashServer = server
}

func (r *Router) V2RayServer() adapter.V2RayServer {
	return r.v2rayServer
}

func (r *Router) SetV2RayServer(server adapter.V2RayServer) {
	r.v2rayServer = server
}

func hasRule(rules []option.Rule, cond func(rule option.DefaultRule) bool) bool {
	for _, rule := range rules {
		switch rule.Type {
		case C.RuleTypeDefault:
			if cond(rule.DefaultOptions) {
				return true
			}
		case C.RuleTypeLogical:
			for _, subRule := range rule.LogicalOptions.Rules {
				if cond(subRule) {
					return true
				}
			}
		}
	}
	return false
}

func hasDNSRule(rules []option.DNSRule, cond func(rule option.DefaultDNSRule) bool) bool {
	for _, rule := range rules {
		switch rule.Type {
		case C.RuleTypeDefault:
			if cond(rule.DefaultOptions) {
				return true
			}
		case C.RuleTypeLogical:
			for _, subRule := range rule.LogicalOptions.Rules {
				if cond(subRule) {
					return true
				}
			}
		}
	}
	return false
}

func isGeoIPRule(rule option.DefaultRule) bool {
	return len(rule.SourceGeoIP) > 0 && common.Any(rule.SourceGeoIP, notPrivateNode) || len(rule.GeoIP) > 0 && common.Any(rule.GeoIP, notPrivateNode)
}

func isGeoIPDNSRule(rule option.DefaultDNSRule) bool {
	return len(rule.SourceGeoIP) > 0 && common.Any(rule.SourceGeoIP, notPrivateNode)
}

func isGeositeRule(rule option.DefaultRule) bool {
	return len(rule.Geosite) > 0
}

func isGeositeDNSRule(rule option.DefaultDNSRule) bool {
	return len(rule.Geosite) > 0
}

func isProcessRule(rule option.DefaultRule) bool {
	return len(rule.ProcessName) > 0 || len(rule.ProcessPath) > 0 || len(rule.PackageName) > 0 || len(rule.User) > 0 || len(rule.UserID) > 0
}

func isProcessDNSRule(rule option.DefaultDNSRule) bool {
	return len(rule.ProcessName) > 0 || len(rule.ProcessPath) > 0 || len(rule.PackageName) > 0 || len(rule.User) > 0 || len(rule.UserID) > 0
}

func notPrivateNode(code string) bool {
	return code != "private"
}

func (r *Router) prepareGeoIPDatabase() error {
	var geoPath string
	if r.geoIPOptions.Path != "" {
		geoPath = r.geoIPOptions.Path
	} else {
		geoPath = "geoip.db"
		if foundPath, loaded := C.FindPath(geoPath); loaded {
			geoPath = foundPath
		}
	}
	geoPath = C.BasePath(geoPath)
	if !rw.FileExists(geoPath) {
		r.logger.Warn("geoip database not exists: ", geoPath)
		var err error
		for attempts := 0; attempts < 3; attempts++ {
			err = r.downloadGeoIPDatabase(geoPath)
			if err == nil {
				break
			}
			r.logger.Error("download geoip database: ", err)
			os.Remove(geoPath)
			// time.Sleep(10 * time.Second)
		}
		if err != nil {
			return err
		}
	}
	geoReader, codes, err := geoip.Open(geoPath)
	if err != nil {
		return E.Cause(err, "open geoip database")
	}
	r.logger.Info("loaded geoip database: ", len(codes), " codes")
	r.geoIPReader = geoReader
	return nil
}

func (r *Router) prepareGeositeDatabase() error {
	var geoPath string
	if r.geositeOptions.Path != "" {
		geoPath = r.geositeOptions.Path
	} else {
		geoPath = "geosite.db"
		if foundPath, loaded := C.FindPath(geoPath); loaded {
			geoPath = foundPath
		}
	}
	geoPath = C.BasePath(geoPath)
	if !rw.FileExists(geoPath) {
		r.logger.Warn("geosite database not exists: ", geoPath)
		var err error
		for attempts := 0; attempts < 3; attempts++ {
			err = r.downloadGeositeDatabase(geoPath)
			if err == nil {
				break
			}
			r.logger.Error("download geosite database: ", err)
			os.Remove(geoPath)
			// time.Sleep(10 * time.Second)
		}
		if err != nil {
			return err
		}
	}
	geoReader, codes, err := geosite.Open(geoPath)
	if err == nil {
		r.logger.Info("loaded geosite database: ", len(codes), " codes")
		r.geositeReader = geoReader
	} else {
		return E.Cause(err, "open geosite database")
	}
	return nil
}

func (r *Router) downloadGeoIPDatabase(savePath string) error {
	var downloadURL string
	if r.geoIPOptions.DownloadURL != "" {
		downloadURL = r.geoIPOptions.DownloadURL
	} else {
		downloadURL = "https://github.com/SagerNet/sing-geoip/releases/latest/download/geoip.db"
	}
	r.logger.Info("downloading geoip database")
	var detour adapter.Outbound
	if r.geoIPOptions.DownloadDetour != "" {
		outbound, loaded := r.Outbound(r.geoIPOptions.DownloadDetour)
		if !loaded {
			return E.New("detour outbound not found: ", r.geoIPOptions.DownloadDetour)
		}
		detour = outbound
	} else {
		detour = r.defaultOutboundForConnection
	}

	if parentDir := filepath.Dir(savePath); parentDir != "" {
		os.MkdirAll(parentDir, 0o755)
	}

	saveFile, err := os.OpenFile(savePath, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return E.Cause(err, "open output file: ", downloadURL)
	}
	defer saveFile.Close()

	httpClient := &http.Client{
		Transport: &http.Transport{
			ForceAttemptHTTP2:   true,
			TLSHandshakeTimeout: 5 * time.Second,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return detour.DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
		},
	}
	defer httpClient.CloseIdleConnections()
	response, err := httpClient.Get(downloadURL)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, err = io.Copy(saveFile, response.Body)
	return err
}

func (r *Router) downloadGeositeDatabase(savePath string) error {
	var downloadURL string
	if r.geositeOptions.DownloadURL != "" {
		downloadURL = r.geositeOptions.DownloadURL
	} else {
		downloadURL = "https://github.com/SagerNet/sing-geosite/releases/latest/download/geosite.db"
	}
	r.logger.Info("downloading geosite database")
	var detour adapter.Outbound
	if r.geositeOptions.DownloadDetour != "" {
		outbound, loaded := r.Outbound(r.geositeOptions.DownloadDetour)
		if !loaded {
			return E.New("detour outbound not found: ", r.geositeOptions.DownloadDetour)
		}
		detour = outbound
	} else {
		detour = r.defaultOutboundForConnection
	}

	if parentDir := filepath.Dir(savePath); parentDir != "" {
		os.MkdirAll(parentDir, 0o755)
	}

	saveFile, err := os.OpenFile(savePath, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return E.Cause(err, "open output file: ", downloadURL)
	}
	defer saveFile.Close()

	httpClient := &http.Client{
		Transport: &http.Transport{
			ForceAttemptHTTP2:   true,
			TLSHandshakeTimeout: 5 * time.Second,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return detour.DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
		},
	}
	defer httpClient.CloseIdleConnections()
	response, err := httpClient.Get(downloadURL)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, err = io.Copy(saveFile, response.Body)
	return err
}

func (r *Router) OnPackagesUpdated(packages int, sharedUsers int) {
	r.logger.Info("updated packages list: ", packages, " packages, ", sharedUsers, " shared users")
}

func (r *Router) NewError(ctx context.Context, err error) {
	common.Close(err)
	if E.IsClosedOrCanceled(err) {
		r.logger.DebugContext(ctx, "connection closed: ", err)
		return
	}
	r.logger.ErrorContext(ctx, err)
}

func (r *Router) notifyNetworkUpdate(int) error {
	if C.IsAndroid {
		var vpnStatus string
		if r.interfaceMonitor.AndroidVPNEnabled() {
			vpnStatus = "enabled"
		} else {
			vpnStatus = "disabled"
		}
		r.logger.Info("updated default interface ", r.interfaceMonitor.DefaultInterfaceName(netip.IPv4Unspecified()), ", index ", r.interfaceMonitor.DefaultInterfaceIndex(netip.IPv4Unspecified()), ", vpn ", vpnStatus)
	} else {
		r.logger.Info("updated default interface ", r.interfaceMonitor.DefaultInterfaceName(netip.IPv4Unspecified()), ", index ", r.interfaceMonitor.DefaultInterfaceIndex(netip.IPv4Unspecified()))
	}

	for _, outbound := range r.outbounds {
		listener, isListener := outbound.(adapter.InterfaceUpdateListener)
		if isListener {
			err := listener.InterfaceUpdated()
			if err != nil {
				return err
			}
		}
	}
	return nil
}
