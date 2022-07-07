package outbound

import (
	"context"
	"net"

	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/sagernet/sing-shadowsocks"
	"github.com/sagernet/sing-shadowsocks/shadowimpl"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

var _ adapter.Outbound = (*Shadowsocks)(nil)

type Shadowsocks struct {
	myOutboundAdapter
	dialer     N.Dialer
	method     shadowsocks.Method
	serverAddr M.Socksaddr
}

func NewShadowsocks(router adapter.Router, logger log.Logger, tag string, options option.ShadowsocksOutboundOptions) (*Shadowsocks, error) {
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
		dialer.New(router, options.DialerOptions),
		method,
		options.ServerOptions.Build(),
	}, nil
}

func (h *Shadowsocks) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.AppendContext(ctx)
	metadata.Outbound = h.tag
	switch network {
	case C.NetworkTCP:
		h.logger.WithContext(ctx).Info("outbound connection to ", destination)
		outConn, err := h.dialer.DialContext(ctx, C.NetworkTCP, h.serverAddr)
		if err != nil {
			return nil, err
		}
		return h.method.DialEarlyConn(outConn, destination), nil
	case C.NetworkUDP:
		h.logger.WithContext(ctx).Info("outbound packet connection to ", destination)
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
	h.logger.WithContext(ctx).Info("outbound packet connection to ", h.serverAddr)
	outConn, err := h.dialer.ListenPacket(ctx, destination)
	if err != nil {
		return nil, err
	}
	return h.method.DialPacketConn(&bufio.BindPacketConn{PacketConn: outConn, Addr: h.serverAddr.UDPAddr()}), nil
}

func (h *Shadowsocks) NewConnection(ctx context.Context, conn net.Conn, destination M.Socksaddr) error {
	serverConn, err := h.DialContext(ctx, C.NetworkTCP, destination)
	if err != nil {
		return err
	}
	return CopyEarlyConn(ctx, conn, serverConn)
}

func (h *Shadowsocks) NewPacketConnection(ctx context.Context, conn N.PacketConn, destination M.Socksaddr) error {
	serverConn, err := h.ListenPacket(ctx, destination)
	if err != nil {
		return err
	}
	return bufio.CopyPacketConn(ctx, conn, bufio.NewPacketConn(serverConn))
}
