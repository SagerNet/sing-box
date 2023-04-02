package outbound

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/mux"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/v2ray"
	"github.com/sagernet/sing-vmess"
	"github.com/sagernet/sing-vmess/packetaddr"
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
	tlsConfig       tls.Config
	transport       adapter.V2RayClientTransport
	packetAddr      bool
	xudp            bool
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
		dialer:     dialer.New(router, options.DialerOptions),
		serverAddr: options.ServerOptions.Build(),
	}
	var err error
	if options.TLS != nil {
		outbound.tlsConfig, err = tls.NewClient(router, options.Server, common.PtrValueOrDefault(options.TLS))
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
	switch options.PacketEncoding {
	case "":
	case "packetaddr":
		outbound.packetAddr = true
	case "xudp":
		outbound.xudp = true
	default:
		return nil, E.New("unknown packet encoding: ", options.PacketEncoding)
	}
	var clientOptions []vmess.ClientOption
	if timeFunc := router.TimeFunc(); timeFunc != nil {
		clientOptions = append(clientOptions, vmess.ClientWithTimeFunc(timeFunc))
	}
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
	return common.Close(h.multiplexDialer, h.transport)
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
	return NewConnection(ctx, h, conn, metadata)
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
			conn, err = tls.ClientHandshake(ctx, conn, h.tlsConfig)
		}
	}
	if err != nil {
		common.Close(conn)
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
			conn, err = tls.ClientHandshake(ctx, conn, h.tlsConfig)
		}
	}
	if err != nil {
		return nil, err
	}
	if h.packetAddr {
		if destination.IsFqdn() {
			return nil, E.New("packetaddr: domain destination is not supported")
		}
		return packetaddr.NewConn(h.client.DialEarlyPacketConn(conn, M.Socksaddr{Fqdn: packetaddr.SeqPacketMagicAddress}), destination), nil
	} else if h.xudp {
		return h.client.DialEarlyXUDPPacketConn(conn, destination), nil
	} else {
		return h.client.DialEarlyPacketConn(conn, destination), nil
	}
}
