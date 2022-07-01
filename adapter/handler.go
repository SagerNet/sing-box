package adapter

import (
	"context"
	"net"

	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type (
	ConnectionHandler interface {
		NewConnection(ctx context.Context, conn net.Conn, metadata InboundContext) error
	}
	PacketHandler interface {
		NewPacket(ctx context.Context, conn N.PacketConn, buffer *buf.Buffer, metadata InboundContext) error
	}
	PacketConnectionHandler interface {
		NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata InboundContext) error
	}
	UpstreamHandlerAdapter interface {
		N.TCPConnectionHandler
		N.UDPConnectionHandler
		E.Handler
	}
	ConnectionHandlerFunc       = func(ctx context.Context, conn net.Conn, metadata InboundContext) error
	PacketConnectionHandlerFunc = func(ctx context.Context, conn N.PacketConn, metadata InboundContext) error
)

func NewUpstreamHandler(
	metadata InboundContext,
	connectionHandler ConnectionHandlerFunc,
	packetHandler PacketConnectionHandlerFunc,
	errorHandler E.Handler,
) UpstreamHandlerAdapter {
	return &myUpstreamHandlerWrapper{
		metadata:          metadata,
		connectionHandler: connectionHandler,
		packetHandler:     packetHandler,
		errorHandler:      errorHandler,
	}
}

var _ UpstreamHandlerAdapter = (*myUpstreamHandlerWrapper)(nil)

type myUpstreamHandlerWrapper struct {
	metadata          InboundContext
	connectionHandler ConnectionHandlerFunc
	packetHandler     PacketConnectionHandlerFunc
	errorHandler      E.Handler
}

func (w *myUpstreamHandlerWrapper) NewConnection(ctx context.Context, conn net.Conn, metadata M.Metadata) error {
	w.metadata.Destination = metadata.Destination
	return w.connectionHandler(ctx, conn, w.metadata)
}

func (w *myUpstreamHandlerWrapper) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata M.Metadata) error {
	w.metadata.Destination = metadata.Destination
	return w.packetHandler(ctx, conn, w.metadata)
}

func (w *myUpstreamHandlerWrapper) NewError(ctx context.Context, err error) {
	w.errorHandler.NewError(ctx, err)
}

var myContextType = (*MetadataContext)(nil)

type MetadataContext struct {
	context.Context
	Metadata InboundContext
}

func (c *MetadataContext) Value(key any) any {
	if key == myContextType {
		return c
	}
	return c.Context.Value(key)
}

type myUpstreamContextHandlerWrapper struct {
	connectionHandler ConnectionHandlerFunc
	packetHandler     PacketConnectionHandlerFunc
	errorHandler      E.Handler
}

func NewUpstreamContextHandler(
	connectionHandler ConnectionHandlerFunc,
	packetHandler PacketConnectionHandlerFunc,
	errorHandler E.Handler,
) UpstreamHandlerAdapter {
	return &myUpstreamContextHandlerWrapper{
		connectionHandler: connectionHandler,
		packetHandler:     packetHandler,
		errorHandler:      errorHandler,
	}
}

func (w *myUpstreamContextHandlerWrapper) NewConnection(ctx context.Context, conn net.Conn, metadata M.Metadata) error {
	myCtx := ctx.Value(myContextType).(*MetadataContext)
	ctx = myCtx.Context
	myCtx.Metadata.Destination = metadata.Destination
	return w.connectionHandler(ctx, conn, myCtx.Metadata)
}

func (w *myUpstreamContextHandlerWrapper) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata M.Metadata) error {
	myCtx := ctx.Value(myContextType).(*MetadataContext)
	ctx = myCtx.Context
	myCtx.Metadata.Destination = metadata.Destination
	return w.packetHandler(ctx, conn, myCtx.Metadata)
}

func (w *myUpstreamContextHandlerWrapper) NewError(ctx context.Context, err error) {
	w.errorHandler.NewError(ctx, err)
}
