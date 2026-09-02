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
	"github.com/sagernet/sing/common"
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
		ConnectionDialer: &routerDialer{ctx: ctx, router: router, tag: tag},
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
		Adapter:    inbound.NewAdapter(C.TypeCloudflared, tag),
		service:    service,
		references: common.FilterNotDefault([]string{options.ControlDialer.Detour, options.TunnelDialer.Detour}),
	}, nil
}

type Inbound struct {
	inbound.Adapter
	service    *cloudflared.Service
	references []string
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
	ctx    context.Context
	router adapter.Router
	tag    string
}

// cloudflared cancels the dial context as soon as DialContext/ListenPacket returns
// (router_pipe.go dialRouterTCPWithMetadata, origin_dial.go dialWarpPacketConnection), while the
// routing and the outbound dial behind the pipe are still in progress, so only its deadline is
// honoured, and only until the outbound handshake is reported.
func (d *routerDialer) routeContext(ctx context.Context) (context.Context, func(), func()) {
	routeCtx, cancel := context.WithCancelCause(context.WithoutCancel(ctx))
	stopShutdown := context.AfterFunc(d.ctx, func() {
		cancel(context.Cause(d.ctx))
	})
	var connectTimeout *time.Timer
	if deadline, hasDeadline := ctx.Deadline(); hasDeadline {
		connectTimeout = time.AfterFunc(time.Until(deadline), func() {
			cancel(context.DeadlineExceeded)
		})
	}
	connected := func() {
		if connectTimeout != nil {
			connectTimeout.Stop()
		}
	}
	closed := func() {
		connected()
		stopShutdown()
	}
	return routeCtx, connected, closed
}

type routedConn struct {
	net.Conn
	connected func()
}

func (c *routedConn) HandshakeSuccess() error {
	c.connected()
	return nil
}

func (c *routedConn) ReaderReplaceable() bool {
	return true
}

func (c *routedConn) WriterReplaceable() bool {
	return true
}

func (c *routedConn) Upstream() any {
	return c.Conn
}

type routedPacketConn struct {
	N.PacketConn
	connected func()
}

func (c *routedPacketConn) HandshakeSuccess() error {
	c.connected()
	return nil
}

func (c *routedPacketConn) ReaderReplaceable() bool {
	return true
}

func (c *routedPacketConn) WriterReplaceable() bool {
	return true
}

func (c *routedPacketConn) Upstream() any {
	return c.PacketConn
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
	routeCtx, connected, closed := d.routeContext(ctx)
	go d.router.RouteConnectionEx(routeCtx, &routedConn{Conn: output, connected: connected}, d.newMetadata(N.NetworkTCP, destination), N.OnceClose(func(it error) {
		closed()
		input.Close()
	}))
	return input, nil
}

func (d *routerDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	input, output := pipe.Pipe()
	routeCtx, connected, closed := d.routeContext(ctx)
	routerConn := &routedPacketConn{PacketConn: bufio.NewUnbindPacketConn(output), connected: connected}
	go d.router.RoutePacketConnectionEx(routeCtx, routerConn, d.newMetadata(N.NetworkUDP, destination), N.OnceClose(func(it error) {
		closed()
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
	}, nil)
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

func (i *Inbound) References() []string {
	return i.references
}

var (
	_ N.HandshakeSuccess   = (*routedConn)(nil)
	_ N.ReaderWithUpstream = (*routedConn)(nil)
	_ N.WriterWithUpstream = (*routedConn)(nil)
	_ N.HandshakeSuccess   = (*routedPacketConn)(nil)
	_ N.ReaderWithUpstream = (*routedPacketConn)(nil)
	_ N.WriterWithUpstream = (*routedPacketConn)(nil)
)
