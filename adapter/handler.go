package adapter

import (
	"context"
	"net"

	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

// Deprecated
type ConnectionHandler interface {
	NewConnection(ctx context.Context, conn net.Conn, metadata InboundContext) error
}

type ConnectionHandlerEx interface {
	NewConnectionEx(ctx context.Context, conn net.Conn, metadata InboundContext, onClose N.CloseHandlerFunc)
}

// Deprecated: use PacketHandlerEx instead
type PacketHandler interface {
	NewPacket(ctx context.Context, conn N.PacketConn, buffer *buf.Buffer, metadata InboundContext) error
}

type PacketHandlerEx interface {
	NewPacketEx(buffer *buf.Buffer, source M.Socksaddr)
}

// Deprecated: use OOBPacketHandlerEx instead
type OOBPacketHandler interface {
	NewPacket(ctx context.Context, conn N.PacketConn, buffer *buf.Buffer, oob []byte, metadata InboundContext) error
}

type OOBPacketHandlerEx interface {
	NewPacketEx(buffer *buf.Buffer, oob []byte, source M.Socksaddr)
}

// Deprecated
type PacketConnectionHandler interface {
	NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata InboundContext) error
}

type PacketConnectionHandlerEx interface {
	NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, metadata InboundContext, onClose N.CloseHandlerFunc)
}

// Deprecated: use TCPConnectionHandlerEx instead
//
//nolint:staticcheck
type UpstreamHandlerAdapter interface {
	N.TCPConnectionHandler
	N.UDPConnectionHandler
	E.Handler
}

type UpstreamHandlerAdapterEx interface {
	N.TCPConnectionHandlerEx
	N.UDPConnectionHandlerEx
}
