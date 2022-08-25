package outbound

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/mux"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/v2ray"
	"github.com/sagernet/sing-vmess"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.Outbound = (*VMess)(nil)

type VMess struct {
	myOutboundAdapter
	dialer          N.Dialer
	client          *vmess.Client
	serverAddr      M.Socksaddr
	multiplexDialer N.Dialer
	tlsConfig       *tls.Config
	transport       adapter.V2RayClientTransport
}

func NewVMess(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.VMessOutboundOptions) (*VMess, error) {
	outbound := &VMess{
		myOutboundAdapter: myOutboundAdapter{
			protocol: C.TypeVMess,
			network:  options.Network.Build(),
			router:   router,
			logger:   logger,
			tag:      tag,
		},
		dialer:     dialer.NewOutbound(router, options.OutboundDialerOptions),
		serverAddr: options.ServerOptions.Build(),
	}
	var err error
	if options.TLS != nil {
		outbound.tlsConfig, err = dialer.TLSConfig(options.Server, common.PtrValueOrDefault(options.TLS))
		if err != nil {
			return nil, err
		}
	}
	if options.Transport != nil {
		outbound.transport, err = v2ray.NewClientTransport(ctx, outbound.dialer, outbound.serverAddr, common.PtrValueOrDefault(options.Transport), outbound.tlsConfig)
		if err != nil {
			return nil, E.Cause(err, "create client transport: ", options.Transport.Type)
		}
	}
	outbound.multiplexDialer, err = mux.NewClientWithOptions(ctx, (*vmessDialer)(outbound), common.PtrValueOrDefault(options.Multiplex))
	if err != nil {
		return nil, err
	}
	var clientOptions []vmess.ClientOption
	if options.GlobalPadding {
		clientOptions = append(clientOptions, vmess.ClientWithGlobalPadding())
	}
	if options.AuthenticatedLength {
		clientOptions = append(clientOptions, vmess.ClientWithAuthenticatedLength())
	}
	security := options.Security
	if security == "" {
		security = "auto"
	}
	if security == "auto" && outbound.tlsConfig != nil {
		security = "zero"
	}
	client, err := vmess.NewClient(options.UUID, security, options.AlterId, clientOptions...)
	if err != nil {
		return nil, err
	}
	outbound.client = client
	return outbound, nil
}

func (h *VMess) Close() error {
	return common.Close(h.transport)
}

func (h *VMess) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if h.multiplexDialer == nil {
		switch N.NetworkName(network) {
		case N.NetworkTCP:
			h.logger.InfoContext(ctx, "outbound connection to ", destination)
		case N.NetworkUDP:
			h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
		}
		return (*vmessDialer)(h).DialContext(ctx, network, destination)
	} else {
		switch N.NetworkName(network) {
		case N.NetworkTCP:
			h.logger.InfoContext(ctx, "outbound multiplex connection to ", destination)
		case N.NetworkUDP:
			h.logger.InfoContext(ctx, "outbound multiplex packet connection to ", destination)
		}
		return h.multiplexDialer.DialContext(ctx, network, destination)
	}
}

func (h *VMess) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	if h.multiplexDialer == nil {
		h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
		return (*vmessDialer)(h).ListenPacket(ctx, destination)
	} else {
		h.logger.InfoContext(ctx, "outbound multiplex packet connection to ", destination)
		return h.multiplexDialer.ListenPacket(ctx, destination)
	}
}

func (h *VMess) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return NewEarlyConnection(ctx, h, conn, metadata)
}

func (h *VMess) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return NewPacketConnection(ctx, h, conn, metadata)
}

type vmessDialer VMess

func (h *vmessDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.AppendContext(ctx)
	metadata.Outbound = h.tag
	metadata.Destination = destination
	var conn net.Conn
	var err error
	if h.transport != nil {
		conn, err = h.transport.DialContext(ctx)
	} else {
		conn, err = h.dialer.DialContext(ctx, N.NetworkTCP, h.serverAddr)
		if err == nil && h.tlsConfig != nil {
			conn, err = dialer.TLSClient(ctx, conn, h.tlsConfig)
		}
	}
	if err != nil {
		return nil, err
	}
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		return h.client.DialEarlyConn(conn, destination), nil
	case N.NetworkUDP:
		return h.client.DialEarlyPacketConn(conn, destination), nil
	default:
		return nil, E.Extend(N.ErrUnknownNetwork, network)
	}
}

func (h *vmessDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	conn, err := h.DialContext(ctx, N.NetworkUDP, destination)
	if err != nil {
		return nil, err
	}
	return conn.(vmess.PacketConn), nil
}
