//go:build with_cloudflared

package cloudflare

import (
	"context"
	"net"
	"net/netip"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-cloudflared"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/pipe"
	"github.com/sagernet/sing/service"
)

func RegisterInbound(registry *inbound.Registry) {
	inbound.Register[option.CloudflaredInboundOptions](registry, C.TypeCloudflared, NewInbound)
}

func NewInbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.CloudflaredInboundOptions) (adapter.Inbound, error) {
	controlDialer, err := dialer.NewWithOptions(dialer.Options{
		Context:          ctx,
		Options:          options.ControlDialer,
		RemoteIsDomain:   true,
		ResolverOnDetour: true,
	})
	if err != nil {
		return nil, E.Cause(err, "build cloudflared control dialer")
	}
	tunnelDialer, err := dialer.NewWithOptions(dialer.Options{
		Context:          ctx,
		Options:          options.TunnelDialer,
		RemoteIsDomain:   true,
		ResolverOnDetour: true,
	})
	if err != nil {
		return nil, E.Cause(err, "build cloudflared tunnel dialer")
	}
	dnsRouter := service.FromContext[adapter.DNSRouter](ctx)
	controlResolver := newRouterResolver(dnsRouter, controlDialer.(dialer.ResolveDialer).QueryOptions())
	tunnelResolver := newRouterResolver(dnsRouter, tunnelDialer.(dialer.ResolveDialer).QueryOptions())

	service, err := cloudflared.NewService(cloudflared.ServiceOptions{
		Logger:           logger,
		ConnectionDialer: &routerDialer{router: router, tag: tag},
		ControlDialer:    controlDialer,
		TunnelDialer:     tunnelDialer,
		ControlResolver:  controlResolver,
		TunnelResolver:   tunnelResolver,
		ICMPHandler:      &icmpRouterHandler{router: router, logger: logger, tag: tag},
		ConnContext: func(connCtx context.Context) context.Context {
			return adapter.WithContext(connCtx, &adapter.InboundContext{
				Inbound:     tag,
				InboundType: C.TypeCloudflared,
			})
		},
		Token:           options.Token,
		HAConnections:   options.HighAvailabilityConnections,
		Protocol:        options.Protocol,
		PostQuantum:     options.PostQuantum,
		EdgeIPVersion:   options.EdgeIPVersion,
		DatagramVersion: options.DatagramVersion,
		GracePeriod:     time.Duration(options.GracePeriod),
		Region:          options.Region,
	})
	if err != nil {
		return nil, err
	}

	return &Inbound{
		Adapter: inbound.NewAdapter(C.TypeCloudflared, tag),
		service: service,
	}, nil
}

type Inbound struct {
	inbound.Adapter
	service *cloudflared.Service
}

func (i *Inbound) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	return i.service.Start()
}

func (i *Inbound) Close() error {
	return i.service.Close()
}

type routerDialer struct {
	router adapter.Router
	tag    string
}

func (d *routerDialer) newMetadata(network string, destination M.Socksaddr) adapter.InboundContext {
	return adapter.InboundContext{
		Inbound:     d.tag,
		InboundType: C.TypeCloudflared,
		Network:     network,
		Destination: destination,
	}
}

func (d *routerDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	input, output := pipe.Pipe()
	go d.router.RouteConnectionEx(ctx, output, d.newMetadata(N.NetworkTCP, destination), N.OnceClose(func(it error) {
		input.Close()
	}))
	return input, nil
}

func (d *routerDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	input, output := pipe.Pipe()
	routerConn := bufio.NewUnbindPacketConn(output)
	go d.router.RoutePacketConnectionEx(ctx, routerConn, d.newMetadata(N.NetworkUDP, destination), N.OnceClose(func(it error) {
		input.Close()
	}))
	return bufio.NewUnbindPacketConn(input), nil
}

type icmpRouterHandler struct {
	router adapter.Router
	logger log.ContextLogger
	tag    string
}

func (h *icmpRouterHandler) RouteICMPFlow(source netip.Addr, destination netip.Addr) (tun.Port, error) {
	result := h.router.PreMatch(adapter.InboundContext{
		Inbound:     h.tag,
		InboundType: C.TypeCloudflared,
		Network:     N.NetworkICMP,
		Source:      M.SocksaddrFrom(source, 0),
		Destination: M.SocksaddrFrom(destination, 0),
	})
	switch result.Action {
	case adapter.PreMatchFlow:
		flowOutbound, isFlowOutbound := result.Outbound.(adapter.FlowOutbound)
		if !isFlowOutbound {
			return nil, E.New("outbound is not a flow outbound")
		}
		if result.Destination.IsValid() && result.Destination.Addr() != destination {
			h.logger.Trace("drop ICMP flow from ", source, " to ", destination, ": destination override is not supported from cloudflared")
			return nil, E.New("destination override is not supported")
		}
		inet4Address, inet6Address := flowOutbound.PortAddresses()
		var portAddress netip.Addr
		if destination.Is4() {
			portAddress = inet4Address
		} else {
			portAddress = inet6Address
		}
		if !portAddress.IsValid() || !portAddress.IsUnspecified() {
			h.logger.Trace("drop ICMP flow from ", source, " to ", destination, ": forwarding ICMP to outbound/", result.Outbound.Type(), "[", result.Outbound.Tag(), "] is not supported from cloudflared")
			return nil, E.New("unsupported flow outbound")
		}
		h.logger.Debug("link ICMP flow from ", source, " to ", destination, " via outbound/", result.Outbound.Type(), "[", result.Outbound.Tag(), "]")
		return flowOutbound, nil
	case adapter.PreMatchReject:
		h.logger.Trace("reject ICMP flow from ", source, " to ", destination)
		return nil, E.New("rejected")
	case adapter.PreMatchDrop:
		return nil, E.New("dropped")
	default:
		h.logger.Trace("drop ICMP flow from ", source, " to ", destination, ": no direct route")
		return nil, E.New("no direct route")
	}
}
