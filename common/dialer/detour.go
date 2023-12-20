package dialer

import (
	"context"
	"net"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type DetourDialer struct {
	router            adapter.Router
	detour            string
	allowNestedDirect bool
	dialer            N.Dialer
	initOnce          sync.Once
	initErr           error
}

func NewDetour(router adapter.Router, detour string, allowNestedDirect bool) N.Dialer {
	return &DetourDialer{
		router:            router,
		detour:            detour,
		allowNestedDirect: allowNestedDirect,
	}
}

func (d *DetourDialer) Start() error {
	_, err := d.Dialer()
	return err
}

func (d *DetourDialer) Dialer() (N.Dialer, error) {
	d.initOnce.Do(func() {
		var (
			dialer adapter.Outbound
			loaded bool
		)
		dialer, loaded = d.router.Outbound(d.detour)
		if !loaded {
			d.initErr = E.New("outbound detour not found: ", d.detour)
		} else if !d.allowNestedDirect && dialer.Type() == C.TypeDirect {
			d.initErr = E.New("using a direct outbound as a detour is illegal")
		} else {
			d.dialer = dialer
		}
	})
	return d.dialer, d.initErr
}

func (d *DetourDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	dialer, err := d.Dialer()
	if err != nil {
		return nil, err
	}
	return dialer.DialContext(ctx, network, destination)
}

func (d *DetourDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	dialer, err := d.Dialer()
	if err != nil {
		return nil, err
	}
	return dialer.ListenPacket(ctx, destination)
}

func (d *DetourDialer) Upstream() any {
	detour, _ := d.Dialer()
	return detour
}
