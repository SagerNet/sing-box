package dialer

import (
	"context"
	"net"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/config"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type LazyDialer struct {
	router   adapter.Router
	options  config.DialerOptions
	dialer   N.Dialer
	initOnce sync.Once
	initErr  error
}

func NewDialer(router adapter.Router, options config.DialerOptions) N.Dialer {
	return &LazyDialer{
		router:  router,
		options: options,
	}
}

func (d *LazyDialer) Dialer() (N.Dialer, error) {
	d.initOnce.Do(func() {
		if d.options.Detour != "" {
			var loaded bool
			d.dialer, loaded = d.router.Outbound(d.options.Detour)
			if !loaded {
				d.initErr = E.New("outbound detour not found: ", d.options.Detour)
			}
		} else {
			d.dialer = newDialer(d.options)
		}
	})
	return d.dialer, d.initErr
}

func (d *LazyDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	dialer, err := d.Dialer()
	if err != nil {
		return nil, err
	}
	return dialer.DialContext(ctx, network, destination)
}

func (d *LazyDialer) ListenPacket(ctx context.Context) (net.PacketConn, error) {
	dialer, err := d.Dialer()
	if err != nil {
		return nil, err
	}
	return dialer.ListenPacket(ctx)
}
