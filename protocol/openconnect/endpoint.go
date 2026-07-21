package openconnect

import (
	"context"
	"net"
	"net/netip"
	"slices"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/endpoint"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	openconnecttransport "github.com/sagernet/sing-box/transport/openconnect"
	"github.com/sagernet/sing-openconnect"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"go4.org/netipx"
)

func RegisterEndpoint(registry *endpoint.Registry) {
	endpoint.Register[option.OpenConnectEndpointOptions](registry, C.TypeOpenConnect, NewEndpoint)
}

type endpointBase struct {
	endpoint.Adapter
	router adapter.Router
	logger log.ContextLogger
}

func (e *endpointBase) SupportsFlow(network string) bool {
	return slices.Contains(e.Network(), network)
}

func (e *endpointBase) newConnection(ctx context.Context, endpoint adapter.Endpoint, localAddresses []netip.Prefix, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	var metadata adapter.InboundContext
	metadata.Inbound = endpoint.Tag()
	metadata.InboundType = endpoint.Type()
	metadata.Source = source
	if isEndpointLocalAddress(localAddresses, destination.Addr) {
		metadata.OriginDestination = destination
		destination.Addr = loopbackAddressFor(destination.Addr)
	}
	metadata.Destination = destination
	e.logger.InfoContext(ctx, "inbound connection from ", source)
	e.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
	e.router.RouteConnectionEx(ctx, conn, metadata, onClose)
}

func (e *endpointBase) newPacketConnection(ctx context.Context, endpoint adapter.Endpoint, localAddresses []netip.Prefix, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	var metadata adapter.InboundContext
	metadata.Inbound = endpoint.Tag()
	metadata.InboundType = endpoint.Type()
	metadata.Source = source
	if isEndpointLocalAddress(localAddresses, destination.Addr) {
		metadata.OriginDestination = destination
		destination.Addr = loopbackAddressFor(destination.Addr)
		conn = bufio.NewNATPacketConn(bufio.NewNetPacketConn(conn), metadata.OriginDestination, destination)
	}
	metadata.Destination = destination
	e.logger.InfoContext(ctx, "inbound packet connection from ", source)
	e.logger.InfoContext(ctx, "inbound packet connection to ", metadata.Destination)
	e.router.RoutePacketConnectionEx(ctx, conn, metadata, onClose)
}

func (e *endpointBase) newDNSPacket(ctx context.Context, endpoint adapter.Endpoint, payload []byte, source M.Socksaddr, destination M.Socksaddr, writer N.PacketWriter) {
	var metadata adapter.InboundContext
	metadata.Inbound = endpoint.Tag()
	metadata.InboundType = endpoint.Type()
	metadata.Network = N.NetworkUDP
	metadata.Source = source
	metadata.Destination = destination
	metadata.Protocol = C.ProtocolDNS
	e.logger.InfoContext(ctx, "inbound DNS packet from ", source)
	e.router.HijackDNSPacket(ctx, payload, writer, metadata)
}

func isEndpointLocalAddress(localAddresses []netip.Prefix, address netip.Addr) bool {
	for _, localPrefix := range localAddresses {
		if address == localPrefix.Addr() {
			return true
		}
	}
	return false
}

func loopbackAddressFor(address netip.Addr) netip.Addr {
	if address.Is4() {
		return netip.AddrFrom4([4]uint8{127, 0, 0, 1})
	}
	return netip.IPv6Loopback()
}

func judgeOpenConnectFlow(router adapter.Router, tag string, endpointType string, localAddresses []netip.Prefix, network uint8, source netip.AddrPort, destination netip.AddrPort, firstPacket []byte) tun.FlowVerdict {
	for _, localPrefix := range localAddresses {
		if destination.Addr() == localPrefix.Addr() {
			return tun.FlowVerdict{Action: tun.ActionAccept}
		}
	}
	return adapter.JudgeFlow(router, tag, endpointType, network, source, destination, firstPacket)
}

func materialSource(name string, inlineValues []string, path string) (openconnect.Material, error) {
	material := openconnect.Material{Path: path}
	if len(inlineValues) > 0 {
		material.Content = []byte(strings.Join(inlineValues, "\n"))
	}
	return material, material.Validate(name)
}

func configurationFromClientEvent(event openconnect.TunnelConfigurationEvent) openconnecttransport.Configuration {
	configuration := event.Configuration
	mtu := configuration.MTU
	if mtu == 0 {
		mtu = openconnecttransport.DefaultMTU
	}
	routes := common.Map(configuration.Routes, func(route openconnect.TunnelRoute) openconnecttransport.Route {
		return openconnecttransport.Route{
			Prefix:  route.Prefix,
			Gateway: route.Gateway,
			Metric:  route.Metric,
		}
	})
	excludedRoutes := common.Map(configuration.ExcludedRoutes, func(route openconnect.TunnelRoute) openconnecttransport.Route {
		return openconnecttransport.Route{
			Prefix:  route.Prefix,
			Gateway: route.Gateway,
			Metric:  route.Metric,
		}
	})
	if configuration.RemoteAddress.IsValid() {
		remoteAddress := configuration.RemoteAddress.Unmap()
		if remoteAddress.Is6() {
			remoteAddress = remoteAddress.WithZone("")
		}
		remoteAddressExcluded := false
		for _, route := range excludedRoutes {
			if route.Prefix.Contains(remoteAddress) {
				remoteAddressExcluded = true
				break
			}
		}
		if !remoteAddressExcluded {
			excludedRoutes = append(excludedRoutes, openconnecttransport.Route{
				Prefix: netip.PrefixFrom(remoteAddress, remoteAddress.BitLen()),
			})
		}
	}
	dnsAddresses := append([]netip.Addr(nil), configuration.DNS...)
	for _, rule := range configuration.SplitDNSRules {
		dnsAddresses = append(dnsAddresses, rule.Servers...)
	}
	for _, dnsAddress := range dnsAddresses {
		if !dnsAddress.IsValid() {
			continue
		}
		dnsAddressExcluded := false
		for _, route := range excludedRoutes {
			if route.Prefix.Contains(dnsAddress) {
				dnsAddressExcluded = true
				break
			}
		}
		if dnsAddressExcluded {
			continue
		}
		dnsAddressIncluded := false
		for _, route := range routes {
			if route.Prefix.Contains(dnsAddress) {
				dnsAddressIncluded = true
				break
			}
		}
		if !dnsAddressIncluded {
			routes = append(routes, openconnecttransport.Route{
				Prefix: netip.PrefixFrom(dnsAddress, dnsAddress.BitLen()),
			})
		}
	}
	splitDNSRules := common.Map(configuration.SplitDNSRules, func(rule openconnect.TunnelSplitDNSRule) openconnecttransport.SplitDNSRule {
		return openconnecttransport.SplitDNSRule{
			Domains: rule.Domains,
			Servers: rule.Servers,
		}
	})
	return openconnecttransport.Configuration{
		MTU:                      mtu,
		Addresses:                configuration.Addresses,
		Routes:                   routes,
		ExcludedRoutes:           excludedRoutes,
		DNS:                      configuration.DNS,
		NBNS:                     configuration.NBNS,
		SearchDomains:            configuration.SearchDomains,
		SplitDNS:                 configuration.SplitDNS,
		SplitDNSRules:            splitDNSRules,
		ProxyAutoConfigURL:       configuration.ProxyAutoConfigURL,
		Banner:                   configuration.Banner,
		TunnelAllDNS:             configuration.TunnelAllDNS,
		ClientBypassProtocol:     configuration.ClientBypassProtocol,
		IdleTimeout:              configuration.IdleTimeout,
		AuthenticationExpiration: configuration.AuthenticationExpiration,
	}
}

func buildIPSet(routes []openconnecttransport.Route, excludedRoutes []openconnecttransport.Route) (*netipx.IPSet, error) {
	var builder netipx.IPSetBuilder
	for _, route := range routes {
		builder.AddPrefix(route.Prefix)
	}
	for _, route := range excludedRoutes {
		builder.RemovePrefix(route.Prefix)
	}
	return builder.IPSet()
}

func buildPreferredDomains(configuration openconnecttransport.Configuration) map[string]bool {
	preferredDomains := make(map[string]bool)
	for _, domain := range configuration.SearchDomains {
		canonicalDomain := canonicalOpenConnectDomain(domain)
		if canonicalDomain != "" {
			preferredDomains[canonicalDomain] = true
		}
	}
	for _, domain := range configuration.SplitDNS {
		canonicalDomain := canonicalOpenConnectDomain(domain)
		if canonicalDomain != "" {
			preferredDomains[canonicalDomain] = true
		}
	}
	for _, rule := range configuration.SplitDNSRules {
		for _, domain := range rule.Domains {
			canonicalDomain := canonicalOpenConnectDomain(domain)
			if canonicalDomain != "" {
				preferredDomains[canonicalDomain] = true
			}
		}
	}
	return preferredDomains
}

func canonicalOpenConnectDomain(domain string) string {
	return strings.ToLower(strings.Trim(strings.TrimSpace(domain), "."))
}

func openConnectDomainMatchesAny(domain string, suffixes map[string]bool) bool {
	for domain != "" {
		if suffixes[domain] {
			return true
		}
		dotIndex := strings.IndexByte(domain, '.')
		if dotIndex == -1 {
			break
		}
		domain = domain[dotIndex+1:]
	}
	return false
}
