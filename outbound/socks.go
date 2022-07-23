package outbound

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"
)

var _ adapter.Outbound = (*Socks)(nil)

type Socks struct {
	myOutboundAdapter
	client *socks.Client
}

func NewSocks(router adapter.Router, logger log.ContextLogger, tag string, options option.SocksOutboundOptions) (*Socks, error) {
	detour := dialer.NewOutbound(router, options.OutboundDialerOptions)
	var version socks.Version
	var err error
	if options.Version != "" {
		version, err = socks.ParseVersion(options.Version)
	} else {
		version = socks.Version5
	}
	if err != nil {
		return nil, err
	}
	return &Socks{
		myOutboundAdapter{
			protocol: C.TypeSocks,
			network:  options.Network.Build(),
			router:   router,
			logger:   logger,
			tag:      tag,
		},
		socks.NewClient(detour, options.ServerOptions.Build(), version, options.Username, options.Password),
	}, nil
}

func (h *Socks) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.AppendContext(ctx)
	metadata.Outbound = h.tag
	metadata.Destination = destination
	switch network {
	case C.NetworkTCP:
		h.logger.InfoContext(ctx, "outbound connection to ", destination)
	case C.NetworkUDP:
		h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	default:
		panic("unknown network " + network)
	}
	return h.client.DialContext(ctx, network, destination)
}

func (h *Socks) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	ctx, metadata := adapter.AppendContext(ctx)
	metadata.Outbound = h.tag
	metadata.Destination = destination
	h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	return h.client.ListenPacket(ctx, destination)
}

func (h *Socks) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return NewConnection(ctx, h, conn, metadata)
}

func (h *Socks) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return NewPacketConnection(ctx, h, conn, metadata)
}
