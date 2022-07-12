package route

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/geoip"
	"github.com/sagernet/sing-box/common/geosite"
	"github.com/sagernet/sing-box/common/iffmonitor"
	"github.com/sagernet/sing-box/common/sniff"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/rw"

	"golang.org/x/net/dns/dnsmessage"
)

var _ adapter.Router = (*Router)(nil)

type Router struct {
	ctx       context.Context
	logger    log.ContextLogger
	dnsLogger log.ContextLogger

	outboundByTag map[string]adapter.Outbound
	rules         []adapter.Rule

	defaultDetour                      string
	defaultOutboundForConnection       adapter.Outbound
	defaultOutboundForPacketConnection adapter.Outbound

	needGeoIPDatabase   bool
	needGeositeDatabase bool
	geoIPOptions        option.GeoIPOptions
	geositeOptions      option.GeositeOptions
	geoIPReader         *geoip.Reader
	geositeReader       *geosite.Reader
	geositeCache        map[string]adapter.Rule

	dnsClient             *dns.Client
	defaultDomainStrategy dns.DomainStrategy
	dnsRules              []adapter.Rule
	defaultTransport      dns.Transport
	transports            []dns.Transport
	transportMap          map[string]dns.Transport

	autoDetectInterface bool
	interfaceMonitor    iffmonitor.InterfaceMonitor
}

func NewRouter(ctx context.Context, logger log.ContextLogger, dnsLogger log.ContextLogger, options option.RouteOptions, dnsOptions option.DNSOptions) (*Router, error) {
	router := &Router{
		ctx:                   ctx,
		logger:                logger,
		dnsLogger:             dnsLogger,
		outboundByTag:         make(map[string]adapter.Outbound),
		rules:                 make([]adapter.Rule, 0, len(options.Rules)),
		dnsRules:              make([]adapter.Rule, 0, len(dnsOptions.Rules)),
		needGeoIPDatabase:     hasGeoRule(options.Rules, isGeoIPRule) || hasGeoDNSRule(dnsOptions.Rules, isGeoIPDNSRule),
		needGeositeDatabase:   hasGeoRule(options.Rules, isGeositeRule) || hasGeoDNSRule(dnsOptions.Rules, isGeositeDNSRule),
		geoIPOptions:          common.PtrValueOrDefault(options.GeoIP),
		geositeOptions:        common.PtrValueOrDefault(options.Geosite),
		geositeCache:          make(map[string]adapter.Rule),
		defaultDetour:         options.Final,
		dnsClient:             dns.NewClient(dns.DomainStrategy(dnsOptions.DNSClientOptions.Strategy), dnsOptions.DNSClientOptions.DisableCache, dnsOptions.DNSClientOptions.DisableExpire),
		defaultDomainStrategy: dns.DomainStrategy(dnsOptions.Strategy),
		autoDetectInterface:   options.AutoDetectInterface,
	}
	for i, ruleOptions := range options.Rules {
		routeRule, err := NewRule(router, logger, ruleOptions)
		if err != nil {
			return nil, E.Cause(err, "parse rule[", i, "]")
		}
		router.rules = append(router.rules, routeRule)
	}
	for i, dnsRuleOptions := range dnsOptions.Rules {
		dnsRule, err := NewDNSRule(router, logger, dnsRuleOptions)
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
	for i, server := range dnsOptions.Servers {
		var tag string
		if server.Tag != "" {
			tag = server.Tag
		} else {
			tag = F.ToString(i)
		}
		transportTags[i] = tag
		transportTagMap[tag] = true
	}
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
			if server.Address != "local" {
				serverURL, err := url.Parse(server.Address)
				if err != nil {
					return nil, err
				}
				serverAddress := serverURL.Hostname()
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
					return nil, E.New("parse dns server[", tag, "]: missing address_resolver")
				}
			}
			transport, err := dns.NewTransport(ctx, detour, server.Address)
			if err != nil {
				return nil, E.Cause(err, "parse dns server[", tag, "]")
			}
			transports[i] = transport
			dummyTransportMap[tag] = transport
			if server.Tag != "" {
				transportMap[server.Tag] = transport
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
		return nil, E.New("found circular reference in dns servers: ", strings.Join(unresolvedTags, " "))
	}
	var defaultTransport dns.Transport
	if options.Final != "" {
		defaultTransport = dummyTransportMap[options.Final]
		if defaultTransport == nil {
			return nil, E.New("default dns server not found: ", options.Final)
		}
	}
	if defaultTransport == nil {
		if len(transports) == 0 {
			transports = append(transports, dns.NewLocalTransport())
		}
		defaultTransport = transports[0]
	}
	router.defaultTransport = defaultTransport
	router.transports = transports
	router.transportMap = transportMap

	if options.AutoDetectInterface {
		monitor, err := iffmonitor.New(router.logger)
		if err != nil {
			return nil, E.Cause(err, "create default interface monitor")
		}
		router.interfaceMonitor = monitor
	}
	return router, nil
}

func (r *Router) Initialize(outbounds []adapter.Outbound, defaultOutbound func() adapter.Outbound) error {
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
		if common.Contains(detour.Network(), C.NetworkTCP) {
			defaultOutboundForConnection = detour
		}
		if common.Contains(detour.Network(), C.NetworkUDP) {
			defaultOutboundForPacketConnection = detour
		}
	}
	var index, packetIndex int
	if defaultOutboundForConnection == nil {
		for i, detour := range outbounds {
			if common.Contains(detour.Network(), C.NetworkTCP) {
				index = i
				defaultOutboundForConnection = detour
				break
			}
		}
	}
	if defaultOutboundForPacketConnection == nil {
		for i, detour := range outbounds {
			if common.Contains(detour.Network(), C.NetworkUDP) {
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
	for _, rule := range r.rules {
		err := rule.Start()
		if err != nil {
			return err
		}
	}
	for _, rule := range r.dnsRules {
		err := rule.Start()
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
	if r.interfaceMonitor != nil {
		err := r.interfaceMonitor.Start()
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Router) Close() error {
	for _, rule := range r.rules {
		err := rule.Close()
		if err != nil {
			return err
		}
	}
	for _, rule := range r.dnsRules {
		err := rule.Close()
		if err != nil {
			return err
		}
	}
	return common.Close(
		common.PtrOrNil(r.geoIPReader),
		r.interfaceMonitor,
	)
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
	if network == C.NetworkTCP {
		return r.defaultOutboundForConnection
	} else {
		return r.defaultOutboundForPacketConnection
	}
}

func (r *Router) RouteConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	if metadata.SniffEnabled {
		_buffer := buf.StackNew()
		defer common.KeepAlive(_buffer)
		buffer := common.Dup(_buffer)
		defer buffer.Release()
		sniffMetadata, err := sniff.PeekStream(ctx, conn, buffer, sniff.TLSClientHello, sniff.HTTPHost)
		if err == nil {
			metadata.Protocol = sniffMetadata.Protocol
			metadata.Domain = sniffMetadata.Domain
			if metadata.SniffOverrideDestination && sniff.IsDomainName(metadata.Domain) {
				metadata.Destination.Fqdn = metadata.Domain
			}
			if metadata.Domain != "" {
				r.logger.DebugContext(ctx, "sniffed protocol: ", metadata.Protocol, ", domain: ", metadata.Domain)
			} else {
				r.logger.DebugContext(ctx, "sniffed protocol: ", metadata.Protocol)
			}
		}
		if !buffer.IsEmpty() {
			conn = bufio.NewCachedConn(conn, buffer)
		}
	}
	if metadata.Destination.IsFqdn() && metadata.DomainStrategy != dns.DomainStrategyAsIS {
		addresses, err := r.Lookup(adapter.WithContext(ctx, &metadata), metadata.Destination.Fqdn, metadata.DomainStrategy)
		if err != nil {
			return err
		}
		metadata.DestinationAddresses = addresses
		r.dnsLogger.DebugContext(ctx, "resolved [", strings.Join(F.MapToString(metadata.DestinationAddresses), " "), "]")
	}
	detour := r.match(ctx, metadata, r.defaultOutboundForConnection)
	if !common.Contains(detour.Network(), C.NetworkTCP) {
		conn.Close()
		return E.New("missing supported outbound, closing connection")
	}
	return detour.NewConnection(ctx, conn, metadata)
}

func (r *Router) RoutePacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	if metadata.SniffEnabled {
		_buffer := buf.StackNewPacket()
		defer common.KeepAlive(_buffer)
		buffer := common.Dup(_buffer)
		defer buffer.Release()
		_, err := conn.ReadPacket(buffer)
		if err != nil {
			return err
		}
		sniffMetadata, err := sniff.PeekPacket(ctx, buffer.Bytes(), sniff.QUICClientHello)
		originDestination := metadata.Destination
		if err == nil {
			metadata.Protocol = sniffMetadata.Protocol
			metadata.Domain = sniffMetadata.Domain
			if metadata.SniffOverrideDestination && sniff.IsDomainName(metadata.Domain) {
				metadata.Destination.Fqdn = metadata.Domain
			}
			if metadata.Domain != "" {
				r.logger.DebugContext(ctx, "sniffed packet protocol: ", metadata.Protocol, ", domain: ", metadata.Domain)
			} else {
				r.logger.DebugContext(ctx, "sniffed packet protocol: ", metadata.Protocol)
			}
		}
		conn = bufio.NewCachedPacketConn(conn, buffer, originDestination)
	}
	if metadata.Destination.IsFqdn() && metadata.DomainStrategy != dns.DomainStrategyAsIS {
		addresses, err := r.Lookup(adapter.WithContext(ctx, &metadata), metadata.Destination.Fqdn, metadata.DomainStrategy)
		if err != nil {
			return err
		}
		metadata.DestinationAddresses = addresses
		r.dnsLogger.DebugContext(ctx, "resolved [", strings.Join(F.MapToString(metadata.DestinationAddresses), " "), "]")
	}
	detour := r.match(ctx, metadata, r.defaultOutboundForPacketConnection)
	if !common.Contains(detour.Network(), C.NetworkUDP) {
		conn.Close()
		return E.New("missing supported outbound, closing packet connection")
	}
	return detour.NewPacketConnection(ctx, conn, metadata)
}

func (r *Router) Exchange(ctx context.Context, message *dnsmessage.Message) (*dnsmessage.Message, error) {
	return r.dnsClient.Exchange(ctx, r.matchDNS(ctx), message)
}

func (r *Router) Lookup(ctx context.Context, domain string, strategy dns.DomainStrategy) ([]netip.Addr, error) {
	return r.dnsClient.Lookup(ctx, r.matchDNS(ctx), domain, strategy)
}

func (r *Router) LookupDefault(ctx context.Context, domain string) ([]netip.Addr, error) {
	return r.dnsClient.Lookup(ctx, r.matchDNS(ctx), domain, r.defaultDomainStrategy)
}

func (r *Router) match(ctx context.Context, metadata adapter.InboundContext, defaultOutbound adapter.Outbound) adapter.Outbound {
	for i, rule := range r.rules {
		if rule.Match(&metadata) {
			detour := rule.Outbound()
			r.logger.DebugContext(ctx, "match[", i, "] ", rule.String(), " => ", detour)
			if outbound, loaded := r.Outbound(detour); loaded {
				return outbound
			}
			r.logger.ErrorContext(ctx, "outbound not found: ", detour)
		}
	}
	return defaultOutbound
}

func (r *Router) matchDNS(ctx context.Context) dns.Transport {
	metadata := adapter.ContextFrom(ctx)
	if metadata == nil {
		r.dnsLogger.WarnContext(ctx, "no context: ", reflect.TypeOf(ctx))
		return r.defaultTransport
	}
	for i, rule := range r.dnsRules {
		if rule.Match(metadata) {
			detour := rule.Outbound()
			r.dnsLogger.DebugContext(ctx, "match[", i, "] ", rule.String(), " => ", detour)
			if transport, loaded := r.transportMap[detour]; loaded {
				return transport
			}
			r.dnsLogger.ErrorContext(ctx, "transport not found: ", detour)
		}
	}
	return r.defaultTransport
}

func (r *Router) AutoDetectInterface() bool {
	return r.autoDetectInterface
}

func (r *Router) DefaultInterfaceName() string {
	if r.interfaceMonitor == nil {
		return ""
	}
	return r.interfaceMonitor.DefaultInterfaceName()
}

func (r *Router) DefaultInterfaceIndex() int {
	if r.interfaceMonitor == nil {
		return 0
	}
	return r.interfaceMonitor.DefaultInterfaceIndex()
}

func hasGeoRule(rules []option.Rule, cond func(rule option.DefaultRule) bool) bool {
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

func hasGeoDNSRule(rules []option.DNSRule, cond func(rule option.DefaultDNSRule) bool) bool {
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
			time.Sleep(10 * time.Second)
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
		geoPath = r.geoIPOptions.Path
	} else {
		geoPath = "geosite.db"
		if foundPath, loaded := C.FindPath(geoPath); loaded {
			geoPath = foundPath
		}
	}
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
			time.Sleep(10 * time.Second)
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
	r.logger.Info("downloading geoip database")
	var detour adapter.Outbound
	if r.geositeOptions.DownloadDetour != "" {
		outbound, loaded := r.Outbound(r.geositeOptions.DownloadDetour)
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
	response, err := httpClient.Get(downloadURL)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, err = io.Copy(saveFile, response.Body)
	return err
}
