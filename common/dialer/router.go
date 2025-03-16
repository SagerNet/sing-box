package dialer

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

type DefaultOutboundDialer struct {
	outbound adapter.OutboundManager
}

func NewDefaultOutbound(ctx context.Context) N.Dialer {
	return &DefaultOutboundDialer{
		outbound: service.FromContext[adapter.OutboundManager](ctx),
	}
}

func (d *DefaultOutboundDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return d.outbound.Default().DialContext(ctx, network, destination)
}

func (d *DefaultOutboundDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return d.outbound.Default().ListenPacket(ctx, destination)
}

func (d *DefaultOutboundDialer) Upstream() any {
	return d.outbound.Default()
}
