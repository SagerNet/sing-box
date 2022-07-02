package outbound

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-shadowsocks"
	"github.com/sagernet/sing-shadowsocks/shadowimpl"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.Outbound = (*Shadowsocks)(nil)

type Shadowsocks struct {
	myOutboundAdapter
	method     shadowsocks.Method
	serverAddr M.Socksaddr
}

func NewShadowsocks(router adapter.Router, logger log.Logger, tag string, options option.ShadowsocksOutboundOptions) (*Shadowsocks, error) {
	outbound := &Shadowsocks{
		myOutboundAdapter: myOutboundAdapter{
			protocol: C.TypeDirect,
			logger:   logger,
			tag:      tag,
			dialer:   NewDialer(router, options.DialerOptions),
		},
	}
	var err error
	outbound.method, err = shadowimpl.FetchMethod(options.Method, options.Password)
	if err != nil {
		return nil, err
	}
	if options.Server == "" {
		return nil, E.New("missing server address")
	} else if options.ServerPort == 0 {
		return nil, E.New("missing server port")
	}
	outbound.serverAddr = M.ParseSocksaddrHostPort(options.Server, options.ServerPort)
	return outbound, nil
}

func (o *Shadowsocks) NewConnection(ctx context.Context, conn net.Conn, destination M.Socksaddr) error {
	serverConn, err := o.DialContext(ctx, "tcp", destination)
	if err != nil {
		return err
	}
	return CopyEarlyConn(ctx, conn, serverConn)
}

func (o *Shadowsocks) NewPacketConnection(ctx context.Context, conn N.PacketConn, destination M.Socksaddr) error {
	serverConn, err := o.ListenPacket(ctx)
	if err != nil {
		return err
	}
	return bufio.CopyPacketConn(ctx, conn, bufio.NewPacketConn(serverConn))
}

func (o *Shadowsocks) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch network {
	case C.NetworkTCP:
		o.logger.WithContext(ctx).Info("outbound connection to ", destination)
		outConn, err := o.dialer.DialContext(ctx, "tcp", o.serverAddr)
		if err != nil {
			return nil, err
		}
		return o.method.DialEarlyConn(outConn, destination), nil
	case C.NetworkUDP:
		o.logger.WithContext(ctx).Info("outbound packet connection to ", destination)
		outConn, err := o.dialer.DialContext(ctx, "udp", o.serverAddr)
		if err != nil {
			return nil, err
		}
		return &bufio.BindPacketConn{PacketConn: o.method.DialPacketConn(outConn), Addr: destination}, nil
	default:
		panic("unknown network " + network)
	}
}

func (o *Shadowsocks) ListenPacket(ctx context.Context) (net.PacketConn, error) {
	o.logger.WithContext(ctx).Info("outbound packet connection to ", o.serverAddr)
	outConn, err := o.dialer.ListenPacket(ctx)
	if err != nil {
		return nil, err
	}
	return o.method.DialPacketConn(&bufio.BindPacketConn{PacketConn: outConn, Addr: o.serverAddr.UDPAddr()}), nil
}
