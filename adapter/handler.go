package adapter

import (
	"context"
	"net"

	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type ConnectionHandler interface {
	NewConnection(ctx context.Context, conn net.Conn, metadata InboundContext, onClose N.CloseHandlerFunc)
}

type PacketHandler interface {
	NewPacket(buffer *buf.Buffer, source M.Socksaddr)
}

type PacketBatchHandler interface {
	NewPacketBatch(buffers []*buf.Buffer, sources []M.Socksaddr)
}

type OOBPacketHandler interface {
	NewPacket(buffer *buf.Buffer, oob []byte, source M.Socksaddr)
}

type PacketConnectionHandler interface {
	NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata InboundContext, onClose N.CloseHandlerFunc)
}

type UpstreamHandlerAdapter interface {
	N.TCPConnectionHandlerEx
	N.UDPConnectionHandlerEx
}
