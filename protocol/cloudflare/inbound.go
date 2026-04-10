//go:build with_cloudflared

package cloudflare

import (
	"context"
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	boxDialer "github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/route/rule"
	cloudflared "github.com/sagernet/sing-cloudflared"
	tun "github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/pipe"
)

func RegisterInbound(registry *inbound.Registry) {
	inbound.Register[option.CloudflaredInboundOptions](registry, C.TypeCloudflared, NewInbound)
}

func NewInbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.CloudflaredInboundOptions) (adapter.Inbound, error) {
	controlDialer, err := boxDialer.NewWithOptions(boxDialer.Options{
		Context:        ctx,
		Options:        options.ControlDialer,
		RemoteIsDomain: true,
	})
	if err != nil {
		return nil, E.Cause(err, "build cloudflared control dialer")
	}
	tunnelDialer, err := boxDialer.NewWithOptions(boxDialer.Options{
		Context:        ctx,
		Options:        options.TunnelDialer,
		RemoteIsDomain: true,
	})
	if err != nil {
		return nil, E.Cause(err, "build cloudflared tunnel dialer")
	}

	service, err := cloudflared.NewService(cloudflared.ServiceOptions{
		Logger:           logger,
		ConnectionDialer: &routerDialer{router: router, tag: tag},
		ControlDialer:    controlDialer,
		TunnelDialer:     tunnelDialer,
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

func (h *icmpRouterHandler) RouteICMPConnection(ctx context.Context, session tun.DirectRouteSession, routeContext tun.DirectRouteContext, timeout time.Duration) (tun.DirectRouteDestination, error) {
	var ipVersion uint8
	if session.Destination.Is4() {
		ipVersion = 4
	} else {
		ipVersion = 6
	}
	destination := M.SocksaddrFrom(session.Destination, 0)
	routeDestination, err := h.router.PreMatch(adapter.InboundContext{
		Inbound:           h.tag,
		InboundType:       C.TypeCloudflared,
		IPVersion:         ipVersion,
		Network:           N.NetworkICMP,
		Source:            M.SocksaddrFrom(session.Source, 0),
		Destination:       destination,
		OriginDestination: destination,
	}, routeContext, timeout, false)
	if err != nil {
		switch {
		case rule.IsBypassed(err):
			err = nil
		case rule.IsRejected(err):
			h.logger.Trace("reject ICMP connection from ", session.Source, " to ", session.Destination)
		default:
			h.logger.Warn(E.Cause(err, "link ICMP connection from ", session.Source, " to ", session.Destination))
		}
	}
	return routeDestination, err
}
