package dialer

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type RouterDialer struct {
	router adapter.Router
}

func NewRouter(router adapter.Router) N.Dialer {
	return &RouterDialer{router: router}
}

func (d *RouterDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return d.router.DefaultOutbound(network).DialContext(ctx, network, destination)
}

func (d *RouterDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return d.router.DefaultOutbound(N.NetworkUDP).ListenPacket(ctx, destination)
}

func (d *RouterDialer) Upstream() any {
	return d.router
}
