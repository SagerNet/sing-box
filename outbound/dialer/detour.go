package dialer

import (
	"context"
	"net"
	"sync"

	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/sagernet/sing-box/adapter"
)

type detourDialer struct {
	router   adapter.Router
	detour   string
	dialer   N.Dialer
	initOnce sync.Once
	initErr  error
}

func NewDetour(router adapter.Router, detour string) N.Dialer {
	return &detourDialer{router: router, detour: detour}
}

func (d *detourDialer) Start() error {
	_, err := d.Dialer()
	return err
}

func (d *detourDialer) Dialer() (N.Dialer, error) {
	d.initOnce.Do(func() {
		var loaded bool
		d.dialer, loaded = d.router.Outbound(d.detour)
		if !loaded {
			d.initErr = E.New("outbound detour not found: ", d.detour)
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
