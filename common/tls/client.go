package tls

import (
	"context"
	"net"
	"os"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func NewDialerFromOptions(router adapter.Router, dialer N.Dialer, serverAddress string, options option.OutboundTLSOptions) (N.Dialer, error) {
	config, err := NewClient(router, serverAddress, options)
	if err != nil {
		return nil, err
	}
	return NewDialer(dialer, config), nil
}

func NewClient(router adapter.Router, serverAddress string, options option.OutboundTLSOptions) (Config, error) {
	return newStdClient(serverAddress, options)
}

func ClientHandshake(ctx context.Context, conn net.Conn, config Config) (net.Conn, error) {
	tlsConn := config.Client(conn)
	ctx, cancel := context.WithTimeout(ctx, C.TCPTimeout)
	defer cancel()
	err := tlsConn.HandshakeContext(ctx)
	return tlsConn, err
}

type Dialer struct {
	dialer N.Dialer
	config Config
}

func NewDialer(dialer N.Dialer, config Config) N.Dialer {
	return &Dialer{dialer, config}
}

func (d *Dialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if network != N.NetworkTCP {
		return nil, os.ErrInvalid
	}
	conn, err := d.dialer.DialContext(ctx, network, destination)
	if err != nil {
		return nil, err
	}
	return ClientHandshake(ctx, conn, d.config)
}

func (d *Dialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}
