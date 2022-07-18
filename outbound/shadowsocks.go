package outbound

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-shadowsocks"
	"github.com/sagernet/sing-shadowsocks/shadowimpl"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.Outbound = (*Shadowsocks)(nil)

type Shadowsocks struct {
	myOutboundAdapter
	dialer     N.Dialer
	method     shadowsocks.Method
	serverAddr M.Socksaddr
}

func NewShadowsocks(router adapter.Router, logger log.ContextLogger, tag string, options option.ShadowsocksOutboundOptions) (*Shadowsocks, error) {
	method, err := shadowimpl.FetchMethod(options.Method, options.Password)
	if err != nil {
		return nil, err
	}
	return &Shadowsocks{
		myOutboundAdapter{
			protocol: C.TypeDirect,
			logger:   logger,
			tag:      tag,
			network:  options.Network.Build(),
		},
		dialer.NewOutbound(router, options.OutboundDialerOptions),
		method,
		options.ServerOptions.Build(),
	}, nil
}

func (h *Shadowsocks) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
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
		return h.method.DialEarlyConn(outConn, destination), nil
	case C.NetworkUDP:
		h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
		outConn, err := h.dialer.DialContext(ctx, C.NetworkUDP, h.serverAddr)
		if err != nil {
			return nil, err
		}
		return &bufio.BindPacketConn{PacketConn: h.method.DialPacketConn(outConn), Addr: destination}, nil
	default:
		panic("unknown network " + network)
	}
}

func (h *Shadowsocks) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	ctx, metadata := adapter.AppendContext(ctx)
	metadata.Outbound = h.tag
	metadata.Destination = destination
	h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	outConn, err := h.dialer.DialContext(ctx, "udp", h.serverAddr)
	if err != nil {
		return nil, err
	}
	return h.method.DialPacketConn(outConn), nil
}

func (h *Shadowsocks) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return NewEarlyConnection(ctx, h, conn, metadata)
}

func (h *Shadowsocks) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return NewPacketConnection(ctx, h, conn, metadata)
}
