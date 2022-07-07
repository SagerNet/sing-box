package dialer

import (
	"context"
	"crypto/tls"
	"net"

	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/uot"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

var _ N.Dialer = (*OverrideDialer)(nil)

type OverrideDialer struct {
	upstream   N.Dialer
	tlsEnabled bool
	tlsConfig  tls.Config
	uotEnabled bool
}

func NewOverride(upstream N.Dialer, options option.OverrideStreamOptions) N.Dialer {
	return &OverrideDialer{
		upstream,
		options.TLS,
		tls.Config{
			ServerName:         options.TLSServerName,
			InsecureSkipVerify: options.TLSInsecure,
		},
		options.UDPOverTCP,
	}
}

func (d *OverrideDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch network {
	case C.NetworkTCP:
		conn, err := d.upstream.DialContext(ctx, C.NetworkTCP, destination)
		if err != nil {
			return nil, err
		}
		return tls.Client(conn, &d.tlsConfig), nil
	case C.NetworkUDP:
		if d.uotEnabled {
			tcpConn, err := d.upstream.DialContext(ctx, C.NetworkTCP, destination)
			if err != nil {
				return nil, err
			}
			return uot.NewClientConn(tcpConn), nil
		}
	}
	return d.upstream.DialContext(ctx, network, destination)
}

func (d *OverrideDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	if d.uotEnabled {
		tcpConn, err := d.upstream.DialContext(ctx, C.NetworkTCP, destination)
		if err != nil {
			return nil, err
		}
		return uot.NewClientConn(tcpConn), nil
	}
	return d.upstream.ListenPacket(ctx, destination)
}

func (d *OverrideDialer) Upstream() any {
	return d.upstream
}
