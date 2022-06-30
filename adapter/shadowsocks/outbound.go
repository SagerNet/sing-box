package shadowsocks

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/tunnel"
	"github.com/sagernet/sing-box/config"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-shadowsocks"
	"github.com/sagernet/sing-shadowsocks/shadowimpl"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.Outbound = (*Outbound)(nil)

type Outbound struct {
	tag        string
	logger     log.Logger
	dialer     N.Dialer
	method     shadowsocks.Method
	serverAddr M.Socksaddr
}

func NewOutbound(tag string, router adapter.Router, logger log.Logger, options *config.ShadowsocksOutboundOptions) (outbound *Outbound, err error) {
	outbound = &Outbound{
		tag:    tag,
		logger: logger,
		dialer: dialer.NewDialer(router, options.DialerOptions),
	}
	outbound.method, err = shadowimpl.FetchMethod(options.Method, options.Password)
	if err != nil {
		return
	}
	if options.Server == "" {
		err = E.New("missing server address")
		return
	} else if options.ServerPort == 0 {
		err = E.New("missing server port")
		return
	}
	outbound.serverAddr = M.ParseSocksaddrHostPort(options.Server, options.ServerPort)
	return
}

func (o *Outbound) Type() string {
	return C.TypeShadowsocks
}

func (o *Outbound) Tag() string {
	return o.tag
}

func (o *Outbound) NewConnection(ctx context.Context, conn net.Conn, destination M.Socksaddr) error {
	serverConn, err := o.DialContext(ctx, "tcp", destination)
	if err != nil {
		return err
	}
	return tunnel.CopyEarlyConn(ctx, conn, serverConn)
}

func (o *Outbound) NewPacketConnection(ctx context.Context, conn N.PacketConn, destination M.Socksaddr) error {
	serverConn, err := o.ListenPacket(ctx)
	if err != nil {
		return err
	}
	return bufio.CopyPacketConn(ctx, conn, bufio.NewPacketConn(serverConn))
}

func (o *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch network {
	case C.NetworkTCP:
		o.logger.WithContext(ctx).Debug("outbound connection to ", destination)
		outConn, err := o.dialer.DialContext(ctx, "tcp", o.serverAddr)
		if err != nil {
			return nil, err
		}
		return o.method.DialEarlyConn(outConn, destination), nil
	case C.NetworkUDP:
		o.logger.WithContext(ctx).Debug("outbound packet connection to ", destination)
		outConn, err := o.dialer.DialContext(ctx, "udp", o.serverAddr)
		if err != nil {
			return nil, err
		}
		return &bufio.BindPacketConn{PacketConn: o.method.DialPacketConn(outConn), Addr: destination}, nil
	default:
		panic("unknown network " + network)
	}
}

func (o *Outbound) ListenPacket(ctx context.Context) (net.PacketConn, error) {
	o.logger.WithContext(ctx).Debug("outbound packet connection to ", o.serverAddr)
	outConn, err := o.dialer.ListenPacket(ctx)
	if err != nil {
		return nil, err
	}
	return o.method.DialPacketConn(&bufio.BindPacketConn{PacketConn: outConn, Addr: o.serverAddr.UDPAddr()}), nil
}
