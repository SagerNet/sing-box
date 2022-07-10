package adapter

import (
	"context"
	"net"

	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type (
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

func UpstreamMetadata(metadata InboundContext) M.Metadata {
	return M.Metadata{
		Source:      metadata.Source,
		Destination: metadata.Destination,
	}
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
	myMetadata := ContextFrom(ctx)
	myMetadata.Destination = metadata.Destination
	return w.connectionHandler(ctx, conn, *myMetadata)
}

func (w *myUpstreamContextHandlerWrapper) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata M.Metadata) error {
	myMetadata := ContextFrom(ctx)
	myMetadata.Destination = metadata.Destination
	return w.packetHandler(ctx, conn, *myMetadata)
}

func (w *myUpstreamContextHandlerWrapper) NewError(ctx context.Context, err error) {
	w.errorHandler.NewError(ctx, err)
}
