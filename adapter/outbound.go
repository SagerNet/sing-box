package adapter

import (
	"context"
	"net"

	N "github.com/sagernet/sing/common/network"
)

type Outbound interface {
	Type() string
	Tag() string
	Network() []string
	N.Dialer
	NewConnection(ctx context.Context, conn net.Conn, metadata InboundContext) error
	NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata InboundContext) error
}
