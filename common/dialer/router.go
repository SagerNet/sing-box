package dialer

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type DefaultOutboundDialer struct {
	outboundManager adapter.OutboundManager
}

func NewDefaultOutbound(outboundManager adapter.OutboundManager) N.Dialer {
	return &DefaultOutboundDialer{outboundManager: outboundManager}
}

func (d *DefaultOutboundDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return d.outboundManager.Default().DialContext(ctx, network, destination)
}

func (d *DefaultOutboundDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return d.outboundManager.Default().ListenPacket(ctx, destination)
}

func (d *DefaultOutboundDialer) Upstream() any {
	return d.outboundManager.Default()
}
