package outbound

import (
	"context"
	"net"
	"net/url"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/mux"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/v2ray"
	"github.com/sagernet/sing-box/transport/wsc"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.Outbound = &WSC{}

var _ N.Dialer = &wscDialer{}

type WSC struct {
	myOutboundAdapter
	dialer          N.Dialer
	serverAddr      metadata.Socksaddr
	multiplexDialer *mux.Client
	tlsConfig       tls.Config
	transport       adapter.V2RayClientTransport
	auth            string
	ruleApplicator  *wsc.WSCRuleApplicator
}

type wscDialer WSC

func NewWSC(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.WSCOutboundOptions) (*WSC, error) {
	outboundDialer, err := dialer.New(router, options.DialerOptions)
	if err != nil {
		return nil, err
	}

	var ruleApplicator *wsc.WSCRuleApplicator = nil
	if len(options.Rules) > 0 {
		if ruleApplicator, err = wsc.NewRuleApplicator(options.Rules); err != nil {
			return nil, err
		}
	}

	outbound := &WSC{
		myOutboundAdapter: myOutboundAdapter{
			protocol:     C.TypeWSC,
			network:      options.Network.Build(),
			router:       router,
			logger:       logger,
			tag:          tag,
			dependencies: withDialerDependency(options.DialerOptions),
		},
		dialer:         outboundDialer,
		auth:           options.Auth,
		ruleApplicator: ruleApplicator,
	}

	if options.TLS != nil {
		outbound.tlsConfig, err = tls.NewClient(ctx, options.Server, common.PtrValueOrDefault(options.TLS))
		if err != nil {
			return nil, err
		}
	}

	outbound.serverAddr = options.ServerOptions.Build()

	if options.Transport != nil {
		outbound.transport, err = v2ray.NewClientTransport(ctx, outbound.dialer, outbound.serverAddr, common.PtrValueOrDefault(options.Transport), outbound.tlsConfig)
		if err != nil {
			return nil, E.Cause(err, "create client transport: ", options.Transport.Type)
		}
	}

	outbound.multiplexDialer, err = mux.NewClientWithOptions((*wscDialer)(outbound), logger, common.PtrValueOrDefault(options.Multiplex))
	if err != nil {
		return nil, err
	}

	return outbound, nil
}

func (wsc *WSC) DialContext(ctx context.Context, network string, destination metadata.Socksaddr) (net.Conn, error) {
	if wsc.multiplexDialer == nil {
		switch N.NetworkName(network) {
		case N.NetworkTCP:
			wsc.logger.InfoContext(ctx, "outbound connection to ", destination)
		case N.NetworkUDP:
			wsc.logger.InfoContext(ctx, "outbound packet connection to ", destination)
		}
		return (*wscDialer)(wsc).DialContext(ctx, network, destination)
	} else {
		switch N.NetworkName(network) {
		case N.NetworkTCP:
			wsc.logger.InfoContext(ctx, "outbound multiplex connection to ", destination)
		case N.NetworkUDP:
			wsc.logger.InfoContext(ctx, "outbound multiplex packet connection to ", destination)
		}
		return wsc.multiplexDialer.DialContext(ctx, network, destination)
	}
}

func (wsc *WSC) ListenPacket(ctx context.Context, destination metadata.Socksaddr) (net.PacketConn, error) {
	if wsc.multiplexDialer == nil {
		wsc.logger.InfoContext(ctx, "outbound packet connection to ", destination)
		return (*wscDialer)(wsc).ListenPacket(ctx, destination)
	} else {
		wsc.logger.InfoContext(ctx, "outbound multiplex packet connection to ", destination)
		return wsc.multiplexDialer.ListenPacket(ctx, destination)
	}
}

func (wsc *WSC) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return NewConnection(ctx, wsc, conn, metadata)
}

func (wsc *WSC) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return NewPacketConnection(ctx, wsc, conn, metadata)
}

func (wsc *WSC) InterfaceUpdated() {
	if wsc.transport != nil {
		wsc.transport.Close()
	}
	if wsc.multiplexDialer != nil {
		wsc.multiplexDialer.Reset()
	}
}

func (wsc *WSC) Close() error {
	return common.Close(common.PtrOrNil(wsc.multiplexDialer))
}

func (dialer *wscDialer) DialContext(ctx context.Context, network string, destination metadata.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = dialer.tag
	metadata.Destination = destination

	ep, netw := destination.String(), network
	if dialer.ruleApplicator != nil {
		ep, netw = dialer.ruleApplicator.ApplyEndpointReplace(ep, netw, wsc.RuleDirectionOutbound)
	}

	params := url.Values{}
	params.Set("auth", dialer.auth)
	params.Set("ep", ep)
	params.Set("net", netw)
	ctx = context.WithValue(ctx, adapter.V2RayExtraOptionsKey, adapter.V2RayExtraOptions{
		QueryParams: params,
	})

	var conn net.Conn
	var err error

	if dialer.transport != nil {
		conn, err = dialer.transport.DialContext(ctx)
	} else {
		conn, err = dialer.dialer.DialContext(ctx, N.NetworkTCP, dialer.serverAddr)
		if err == nil && dialer.tlsConfig != nil {
			conn, err = tls.ClientHandshake(ctx, conn, dialer.tlsConfig)
		}
	}
	if err != nil {
		common.Close(conn)
		return nil, err
	}

	switch N.NetworkName(network) {
	case N.NetworkTCP:
		return wsc.NewClientConn(conn, destination)
	case N.NetworkUDP:
		packetConn, err := wsc.NewClientPacketConn(conn, dialer.ruleApplicator)
		return bufio.NewBindPacketConn(packetConn, destination), err
	default:
		return nil, E.Extend(N.ErrUnknownNetwork, network)
	}
}

func (dialer *wscDialer) ListenPacket(ctx context.Context, destination metadata.Socksaddr) (net.PacketConn, error) {
	conn, err := dialer.DialContext(ctx, N.NetworkUDP, destination)
	if err != nil {
		return nil, err
	}
	return conn.(net.PacketConn), nil
}
