package dialer

import (
	"context"
	"net"
	"time"

	"github.com/database64128/tfo-go"
	"github.com/sagernet/sing-box/config"
	"github.com/sagernet/sing/common/control"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type defaultDialer struct {
	tfo.Dialer
	net.ListenConfig
}

func (d *defaultDialer) DialContext(ctx context.Context, network string, address M.Socksaddr) (net.Conn, error) {
	return d.Dialer.DialContext(ctx, network, address.String())
}

func (d *defaultDialer) ListenPacket(ctx context.Context) (net.PacketConn, error) {
	return d.ListenConfig.ListenPacket(ctx, "udp", "")
}

func newDialer(options config.DialerOptions) N.Dialer {
	var dialer net.Dialer
	var listener net.ListenConfig
	if options.BindInterface != "" {
		dialer.Control = control.Append(dialer.Control, control.BindToInterface(options.BindInterface))
		listener.Control = control.Append(listener.Control, control.BindToInterface(options.BindInterface))
	}
	if options.RoutingMark != 0 {
		dialer.Control = control.Append(dialer.Control, control.RoutingMark(options.RoutingMark))
		listener.Control = control.Append(listener.Control, control.RoutingMark(options.RoutingMark))
	}
	if options.ReuseAddr {
		listener.Control = control.Append(listener.Control, control.ReuseAddr())
	}
	if options.ConnectTimeout != 0 {
		dialer.Timeout = time.Duration(options.ConnectTimeout) * time.Second
	}
	return &defaultDialer{tfo.Dialer{Dialer: dialer, DisableTFO: !options.TCPFastOpen}, listener}
}
