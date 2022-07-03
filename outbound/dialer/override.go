package dialer

import (
	"context"
	"crypto/tls"
	"net"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/uot"
)

var _ N.Dialer = (*overrideDialer)(nil)

type overrideDialer struct {
	upstream   N.Dialer
	tlsEnabled bool
	tlsConfig  tls.Config
	uotEnabled bool
}

func newOverride(upstream N.Dialer, options option.OverrideStreamOptions) N.Dialer {
	if !options.TLS && !options.UDPOverTCP {
		return upstream
	}
	return &overrideDialer{
		upstream,
		options.TLS,
		tls.Config{
			ServerName:         options.TLSServerName,
			InsecureSkipVerify: options.TLSInsecure,
		},
		options.UDPOverTCP,
	}
}

func (d *overrideDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
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

func (d *overrideDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	if d.uotEnabled {
		tcpConn, err := d.upstream.DialContext(ctx, C.NetworkTCP, destination)
		if err != nil {
			return nil, err
		}
		return uot.NewClientConn(tcpConn), nil
	}
	return d.upstream.ListenPacket(ctx, destination)
}
