package adapter

import (
	"context"
	"net"

	N "github.com/sagernet/sing/common/network"
)

type ConnectionManager interface {
	Lifecycle
	NewConnection(ctx context.Context, this N.Dialer, conn net.Conn, metadata InboundContext, onClose N.CloseHandlerFunc)
	NewPacketConnection(ctx context.Context, this N.Dialer, conn N.PacketConn, metadata InboundContext, onClose N.CloseHandlerFunc)
}
