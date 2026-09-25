package masque

import (
	"context"
	"net"
	"net/netip"
	"slices"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/endpoint"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/device"
	"github.com/sagernet/sing-box/transport/masque"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

func RegisterEndpoint(registry *endpoint.Registry) {
	endpoint.Register[option.MASQUEClientEndpointOptions](registry, C.TypeMASQUEClient, NewClientEndpoint)
	endpoint.Register[option.MASQUEServerEndpointOptions](registry, C.TypeMASQUEServer, NewServerEndpoint)
}

type endpointBase struct {
	endpoint.Adapter
	router adapter.Router
	logger log.ContextLogger
}

func newDeviceOptions(ctx context.Context, logger log.ContextLogger, handler tun.Handler, options option.MASQUEEndpointOptions, udpTimeout time.Duration, address []netip.Prefix) *device.Options {
	if udpTimeout == 0 {
		udpTimeout = C.UDPTimeout
	}
	return &device.Options{
		Context:             ctx,
		Logger:              logger,
		System:              options.System,
		Handler:             handler,
		UDPTimeout:          udpTimeout,
		ICMPTimeout:         C.ICMPTimeout,
		UDPMapping:          tun.NATMapping(options.UDPMapping),
		UDPFiltering:        tun.NATFiltering(options.UDPFiltering),
		UDPNATMax:           options.UDPNATMax,
		InterfaceFinder:     service.FromContext[adapter.NetworkManager](ctx).InterfaceFinder(),
		Name:                options.Name,
		NamePrefix:          "masque",
		MTU:                 options.MTU,
		PacketFrontHeadroom: masque.PacketHeadroom,
		Configuration: device.Configuration{
			MTU:     options.MTU,
			Address: address,
		},
	}
}

func (e *endpointBase) SupportsFlow(network string) bool {
	return slices.Contains(e.Network(), network)
}

func (e *endpointBase) newConnection(ctx context.Context, endpoint adapter.Endpoint, localAddresses []netip.Prefix, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	var metadata adapter.InboundContext
	metadata.Inbound = endpoint.Tag()
	metadata.InboundType = endpoint.Type()
	metadata.Source = source
	if isLocalAddress(localAddresses, destination.Addr) {
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
	if isLocalAddress(localAddresses, destination.Addr) {
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

func (e *endpointBase) judgeFlow(endpoint adapter.Endpoint, localAddresses []netip.Prefix, network uint8, source netip.AddrPort, destination netip.AddrPort, firstPacket []byte) tun.FlowVerdict {
	if isLocalAddress(localAddresses, destination.Addr()) {
		return tun.FlowVerdict{Action: tun.ActionAccept}
	}
	return adapter.JudgeFlow(e.router, adapter.InboundContext{Inbound: endpoint.Tag(), InboundType: endpoint.Type()}, network, source, destination, firstPacket)
}

func isLocalAddress(localAddresses []netip.Prefix, address netip.Addr) bool {
	return slices.ContainsFunc(localAddresses, func(localPrefix netip.Prefix) bool {
		return address == localPrefix.Addr()
	})
}

func loopbackAddressFor(address netip.Addr) netip.Addr {
	if address.Is4() {
		return netip.AddrFrom4([4]uint8{127, 0, 0, 1})
	}
	return netip.IPv6Loopback()
}
