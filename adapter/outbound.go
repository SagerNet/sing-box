package adapter

import (
	"context"
	"net"

	"github.com/sagernet/sing-tun"
	N "github.com/sagernet/sing/common/network"
)

// Note: for proxy protocols, outbound creates early connections by default.

type Outbound interface {
	Type() string
	Tag() string
	Network() []string
	N.Dialer
	NewConnection(ctx context.Context, conn net.Conn, metadata InboundContext) error
	NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata InboundContext) error
}

type IPOutbound interface {
	Outbound
	NewIPConnection(ctx context.Context, conn tun.RouteContext, metadata InboundContext) (tun.DirectDestination, error)
}
