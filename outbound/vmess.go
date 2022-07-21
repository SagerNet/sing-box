package outbound

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-vmess"
	"github.com/sagernet/sing/common"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.Outbound = (*VMess)(nil)

type VMess struct {
	myOutboundAdapter
	dialer     N.Dialer
	client     *vmess.Client
	serverAddr M.Socksaddr
}

func NewVMess(router adapter.Router, logger log.ContextLogger, tag string, options option.VMessOutboundOptions) (*VMess, error) {
	var clientOptions []vmess.ClientOption
	if options.GlobalPadding {
		clientOptions = append(clientOptions, vmess.ClientWithGlobalPadding())
	}
	if options.AuthenticatedLength {
		clientOptions = append(clientOptions, vmess.ClientWithAuthenticatedLength())
	}
	client, err := vmess.NewClient(options.UUID, options.Security, options.AlterId, clientOptions...)
	if err != nil {
		return nil, err
	}
	detour, err := dialer.NewTLS(dialer.NewOutbound(router, options.OutboundDialerOptions), options.Server, common.PtrValueOrDefault(options.TLSOptions))
	if err != nil {
		return nil, err
	}
	return &VMess{
		myOutboundAdapter{
			protocol: C.TypeVMess,
			logger:   logger,
			tag:      tag,
			network:  options.Network.Build(),
		},
		detour,
		client,
		options.ServerOptions.Build(),
	}, nil
}

func (h *VMess) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.AppendContext(ctx)
	metadata.Outbound = h.tag
	metadata.Destination = destination
	switch network {
	case C.NetworkTCP:
		h.logger.InfoContext(ctx, "outbound connection to ", destination)
		outConn, err := h.dialer.DialContext(ctx, C.NetworkTCP, h.serverAddr)
		if err != nil {
			return nil, err
		}
		return h.client.DialEarlyConn(outConn, destination), nil
	case C.NetworkUDP:
		h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
		outConn, err := h.dialer.DialContext(ctx, C.NetworkTCP, h.serverAddr)
		if err != nil {
			return nil, err
		}
		return h.client.DialEarlyPacketConn(outConn, destination), nil
	default:
		panic("unknown network " + network)
	}
}

func (h *VMess) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	conn, err := h.DialContext(ctx, C.NetworkUDP, destination)
	if err != nil {
		return nil, err
	}
	return conn.(vmess.PacketConn), nil
}

func (h *VMess) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return NewEarlyConnection(ctx, h, conn, metadata)
}

func (h *VMess) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return NewPacketConnection(ctx, h, conn, metadata)
}
