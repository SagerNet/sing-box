package openvpn

import (
	"context"
	"crypto/subtle"
	"net"
	"net/netip"
	"slices"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/endpoint"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	ovpntransport "github.com/sagernet/sing-box/transport/openvpn"
	ovpn "github.com/sagernet/sing-openvpn"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing-tun/gtcpip/header"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"go4.org/netipx"
)

func RegisterEndpoint(registry *endpoint.Registry) {
	endpoint.Register[option.OpenVPNClientEndpointOptions](registry, C.TypeOpenVPNClient, NewClientEndpoint)
	endpoint.Register[option.OpenVPNServerEndpointOptions](registry, C.TypeOpenVPNServer, NewServerEndpoint)
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

func judgeOpenVPNFlow(router adapter.Router, tag string, endpointType string, localAddresses []netip.Prefix, network uint8, source netip.AddrPort, destination netip.AddrPort, firstPacket []byte) tun.FlowVerdict {
	for _, localPrefix := range localAddresses {
		if destination.Addr() == localPrefix.Addr() {
			return tun.FlowVerdict{Action: tun.ActionAccept}
		}
	}
	return adapter.JudgeFlow(router, tag, endpointType, network, source, destination, firstPacket)
}

func keyDirectionValue(direction string) (int, error) {
	switch direction {
	case "":
		return -1, nil
	case "server":
		return 0, nil
	case "client":
		return 1, nil
	default:
		return 0, E.New("unsupported key direction: ", direction, " (expected \"server\" or \"client\")")
	}
}

func openVPNClientRemoteIsDomain(options option.OpenVPNClientEndpointOptions) bool {
	if options.Server != "" && options.ServerIsDomain() {
		return true
	}
	for _, remoteOptions := range options.Servers {
		if remoteOptions.Build().IsDomain() {
			return true
		}
	}
	return false
}

func materialSource(name string, inlineValues []string, path string) (ovpn.Material, error) {
	material := ovpn.Material{Path: path}
	if len(inlineValues) > 0 {
		material.Content = []byte(strings.Join(inlineValues, "\n"))
	}
	return material, material.Validate(name)
}

func requiredMaterialSource(name string, inlineValues []string, path string) (ovpn.Material, error) {
	material, err := materialSource(name, inlineValues, path)
	if err != nil {
		return ovpn.Material{}, err
	}
	if !material.IsSet() {
		return ovpn.Material{}, E.New("missing `", name, "` or `", name, "_path`")
	}
	return material, nil
}

func configurationFromClientEvent(event ovpn.TunnelConfigurationEvent, logger log.ContextLogger) ovpntransport.Configuration {
	configuration := event.Configuration
	var addresses []netip.Prefix
	addresses = append(addresses, configuration.LocalIPv4...)
	addresses = append(addresses, configuration.LocalIPv6...)
	mtu := configuration.TunMTU
	if mtu == 0 {
		mtu = ovpntransport.DefaultMTU
	}
	var routes []ovpntransport.Route
	inet4DefaultRoute := netip.PrefixFrom(netip.IPv4Unspecified(), 0)
	inet6DefaultRoute := netip.PrefixFrom(netip.IPv6Unspecified(), 0)
	var hasInet4DefaultRoute bool
	var hasInet6DefaultRoute bool
	for _, route := range configuration.IPv4Routes {
		routes = append(routes, ovpntransport.Route{
			Prefix:  route.Prefix,
			Gateway: route.Gateway,
			Metric:  route.Metric,
		})
		if route.Prefix == inet4DefaultRoute {
			hasInet4DefaultRoute = true
		}
	}
	for _, route := range configuration.IPv6Routes {
		routes = append(routes, ovpntransport.Route{
			Prefix:  route.Prefix,
			Gateway: route.Gateway,
			Metric:  route.Metric,
		})
		if route.Prefix == inet6DefaultRoute {
			hasInet6DefaultRoute = true
		}
	}
	var excludedRoutes []ovpntransport.Route
	for _, route := range configuration.ExcludedIPv4Routes {
		excludedRoutes = append(excludedRoutes, ovpntransport.Route{Prefix: route.Prefix, Gateway: route.Gateway, Metric: route.Metric})
	}
	for _, route := range configuration.ExcludedIPv6Routes {
		excludedRoutes = append(excludedRoutes, ovpntransport.Route{Prefix: route.Prefix, Gateway: route.Gateway, Metric: route.Metric})
	}
	if configuration.RedirectGateway {
		if !hasOpenVPNFlag(configuration.RedirectGatewayFlags, "!ipv4") && !hasInet4DefaultRoute {
			if hasOpenVPNFlag(configuration.RedirectGatewayFlags, "def1") {
				for _, prefix := range []netip.Prefix{
					netip.PrefixFrom(netip.IPv4Unspecified(), 1),
					netip.MustParsePrefix("128.0.0.0/1"),
				} {
					if !openVPNRoutesContainPrefix(routes, prefix) {
						routes = append(routes, ovpntransport.Route{Prefix: prefix, Gateway: configuration.VPNGateway, Metric: configuration.RouteMetric})
					}
				}
			} else {
				routes = append(routes, ovpntransport.Route{
					Prefix:  inet4DefaultRoute,
					Gateway: configuration.VPNGateway,
					Metric:  configuration.RouteMetric,
				})
			}
		}
		if hasOpenVPNFlag(configuration.RedirectGatewayFlags, "ipv6") && !hasInet6DefaultRoute {
			for _, prefix := range []netip.Prefix{
				netip.MustParsePrefix("::/3"),
				netip.MustParsePrefix("2000::/4"),
				netip.MustParsePrefix("3000::/4"),
				netip.MustParsePrefix("fc00::/7"),
			} {
				if !openVPNRoutesContainPrefix(routes, prefix) {
					routes = append(routes, ovpntransport.Route{Prefix: prefix, Gateway: configuration.VPNGatewayIPv6, Metric: configuration.RouteMetric})
				}
			}
		}
	}
	if configuration.BlockIPv6 && !hasInet6DefaultRoute {
		routes = append(routes, ovpntransport.Route{
			Prefix:  inet6DefaultRoute,
			Gateway: configuration.VPNGatewayIPv6,
			Metric:  configuration.RouteMetric,
		})
	}
	var dnsAddresses []netip.Addr
	if len(configuration.DNSServers) > 0 {
		servers := slices.Clone(configuration.DNSServers)
		slices.SortFunc(servers, func(left ovpn.TunnelDNSServer, right ovpn.TunnelDNSServer) int {
			return left.Priority - right.Priority
		})
		for _, address := range servers[0].Addresses {
			if address.Addr().IsValid() && !slices.Contains(dnsAddresses, address.Addr()) {
				dnsAddresses = append(dnsAddresses, address.Addr())
			}
		}
	} else {
		dnsAddresses = slices.Clone(configuration.DNS)
	}
	for _, dnsAddress := range dnsAddresses {
		if openVPNRoutesContainAddress(routes, dnsAddress) || openVPNRoutesContainAddress(excludedRoutes, dnsAddress) {
			continue
		}
		gateway := configuration.VPNGateway
		if dnsAddress.Is6() {
			gateway = configuration.VPNGatewayIPv6
		}
		routes = append(routes, ovpntransport.Route{
			Prefix:  netip.PrefixFrom(dnsAddress, dnsAddress.BitLen()),
			Gateway: gateway,
			Metric:  configuration.RouteMetric,
		})
	}
	var ignoredOptions []string
	var notApplicableOptions []string
	for _, flag := range configuration.RedirectGatewayFlags {
		switch strings.ToLower(flag) {
		case "!ipv4", "ipv6", "def1", "local", "autolocal":
		case "bypass-dhcp", "bypass-dns":
			notApplicableOptions = append(notApplicableOptions, "redirect-gateway "+flag)
		default:
			if flag != "" {
				ignoredOptions = append(ignoredOptions, "redirect-gateway "+flag)
			}
		}
	}
	if configuration.BlockOutsideDNS {
		ignoredOptions = append(ignoredOptions, "block-outside-dns")
	}
	for _, dhcpOption := range configuration.DHCPOptions {
		fields := strings.Fields(dhcpOption)
		if len(fields) == 0 || slices.ContainsFunc([]string{"DNS", "DNS6", "DOMAIN", "ADAPTER_DOMAIN_SUFFIX", "DOMAIN-SEARCH", "DOMAIN-ROUTE"}, func(optionName string) bool {
			return strings.EqualFold(fields[0], optionName)
		}) {
			continue
		}
		ignoredOptions = append(ignoredOptions, "dhcp-option "+strings.TrimSpace(dhcpOption))
	}
	if len(ignoredOptions) > 0 && logger != nil {
		logger.Debug("ignored pushed options: ", strings.Join(ignoredOptions, ", "))
	}
	if len(notApplicableOptions) > 0 && logger != nil {
		logger.Debug("pushed options are not applicable: ", strings.Join(notApplicableOptions, ", "))
	}
	return ovpntransport.Configuration{
		MTU:            mtu,
		Address:        addresses,
		Routes:         routes,
		ExcludedRoutes: excludedRoutes,
		DNS:            configuration.DNS,
		DNSServers: common.Map(configuration.DNSServers, func(server ovpn.TunnelDNSServer) ovpntransport.DNSServer {
			return ovpntransport.DNSServer{
				Priority:       server.Priority,
				Addresses:      slices.Clone(server.Addresses),
				ResolveDomains: slices.Clone(server.ResolveDomains),
				DNSSEC:         server.DNSSEC,
				Transport:      server.Transport,
				SNI:            server.SNI,
			}
		}),
		SearchDomains: slices.Clone(configuration.SearchDomains),
		DNSRoutes:     slices.Clone(configuration.DNSRoutes),
		Topology:      configuration.Topology,
		BlockIPv6:     configuration.BlockIPv6,
	}
}

func openVPNRoutesContainAddress(routes []ovpntransport.Route, address netip.Addr) bool {
	for _, route := range routes {
		if route.Prefix.Contains(address) {
			return true
		}
	}
	return false
}

func openVPNRoutesContainPrefix(routes []ovpntransport.Route, prefix netip.Prefix) bool {
	for _, route := range routes {
		if route.Prefix == prefix {
			return true
		}
	}
	return false
}

func buildIPSet(routes []ovpntransport.Route, excludedRoutes []ovpntransport.Route) (*netipx.IPSet, error) {
	var builder netipx.IPSetBuilder
	for _, route := range routes {
		builder.AddPrefix(route.Prefix)
	}
	for _, route := range excludedRoutes {
		builder.RemovePrefix(route.Prefix)
	}
	return builder.IPSet()
}

func hasOpenVPNFlag(flags []string, flag string) bool {
	for _, value := range flags {
		if strings.EqualFold(value, flag) {
			return true
		}
	}
	return false
}

func packetSourceAddress(packet []byte, inet4Address netip.Addr, inet6Address netip.Addr) netip.Addr {
	if header.IPVersion(packet) == header.IPv6Version {
		return inet6Address
	}
	return inet4Address
}

func authenticatorFromUsers(users []auth.User) ovpn.UserPassAuthenticator {
	if len(users) == 0 {
		return nil
	}
	passwordByUsername := make(map[string]string, len(users))
	for _, user := range users {
		passwordByUsername[user.Username] = user.Password
	}
	return func(ctx context.Context, username string, password string) error {
		expectedPassword, found := passwordByUsername[username]
		if !found || subtle.ConstantTimeCompare([]byte(expectedPassword), []byte(password)) != 1 {
			return E.New("invalid username or password")
		}
		return nil
	}
}
