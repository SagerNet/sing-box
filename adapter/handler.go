package adapter

import (
	"context"
	"net"

	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
)

type ConnectionHandler interface {
	NewConnection(ctx context.Context, conn net.Conn, metadata InboundContext) error
}

type PacketHandler interface {
	NewPacket(ctx context.Context, conn N.PacketConn, buffer *buf.Buffer, metadata InboundContext) error
}

type OOBPacketHandler interface {
	NewPacket(ctx context.Context, conn N.PacketConn, buffer *buf.Buffer, oob []byte, metadata InboundContext) error
}

type PacketConnectionHandler interface {
	NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata InboundContext) error
}

type UpstreamHandlerAdapter interface {
	N.TCPConnectionHandler
	N.UDPConnectionHandler
	E.Handler
}
