package outbound

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"
)

var _ adapter.Outbound = (*Socks)(nil)

type Socks struct {
	myOutboundAdapter
	client *socks.Client
}

func NewSocks(router adapter.Router, logger log.Logger, tag string, options option.SocksOutboundOptions) (*Socks, error) {
	dialer := NewDialer(router, options.DialerOptions)
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
			logger:   logger,
			tag:      tag,
			dialer:   dialer,
		},
		socks.NewClient(dialer, M.ParseSocksaddrHostPort(options.Server, options.ServerPort), version, options.Username, options.Password),
	}, nil
}

func (h *Socks) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch network {
	case C.NetworkTCP:
		h.logger.WithContext(ctx).Info("outbound connection to ", destination)
	case C.NetworkUDP:
		h.logger.WithContext(ctx).Info("outbound packet connection to ", destination)
	default:
		panic("unknown network " + network)
	}
	return h.client.DialContext(ctx, network, destination)
}

func (h *Socks) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	h.logger.WithContext(ctx).Info("outbound packet connection to ", destination)
	return h.client.ListenPacket(ctx, destination)
}

func (h *Socks) NewConnection(ctx context.Context, conn net.Conn, destination M.Socksaddr) error {
	outConn, err := h.DialContext(ctx, "tcp", destination)
	if err != nil {
		return err
	}
	return bufio.CopyConn(ctx, conn, outConn)
}

func (h *Socks) NewPacketConnection(ctx context.Context, conn N.PacketConn, destination M.Socksaddr) error {
	outConn, err := h.ListenPacket(ctx, destination)
	if err != nil {
		return err
	}
	return bufio.CopyPacketConn(ctx, conn, bufio.NewPacketConn(outConn))
}
