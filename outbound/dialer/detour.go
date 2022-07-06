package dialer

import (
	"context"
	"net"
	"sync"

	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
)

type detourDialer struct {
	router   adapter.Router
	options  option.DialerOptions
	dialer   N.Dialer
	initOnce sync.Once
	initErr  error
}

func newDetour(router adapter.Router, options option.DialerOptions) N.Dialer {
	return &detourDialer{router: router, options: options}
}

func (d *detourDialer) Dialer() (N.Dialer, error) {
	d.initOnce.Do(func() {
		var loaded bool
		d.dialer, loaded = d.router.Outbound(d.options.Detour)
		if !loaded {
			d.initErr = E.New("outbound detour not found: ", d.options.Detour)
		}
	})
	return d.dialer, d.initErr
}

func (d *detourDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	dialer, err := d.Dialer()
	if err != nil {
		return nil, err
	}
	return dialer.DialContext(ctx, network, destination)
}

func (d *detourDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	dialer, err := d.Dialer()
	if err != nil {
		return nil, err
	}
	return dialer.ListenPacket(ctx, destination)
}
