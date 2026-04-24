package adapter

import (
	"context"
	"net"

	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type (
	ConnectionHandlerFunc       = func(ctx context.Context, conn net.Conn, metadata InboundContext, onClose N.CloseHandlerFunc)
	PacketConnectionHandlerFunc = func(ctx context.Context, conn N.PacketConn, metadata InboundContext, onClose N.CloseHandlerFunc)
)

func NewUpstreamHandler(
	metadata InboundContext,
	connectionHandler ConnectionHandlerFunc,
	packetHandler PacketConnectionHandlerFunc,
) UpstreamHandlerAdapter {
	return &myUpstreamHandlerWrapper{
		metadata:          metadata,
		connectionHandler: connectionHandler,
		packetHandler:     packetHandler,
	}
}

var _ UpstreamHandlerAdapter = (*myUpstreamHandlerWrapper)(nil)

type myUpstreamHandlerWrapper struct {
	metadata          InboundContext
	connectionHandler ConnectionHandlerFunc
	packetHandler     PacketConnectionHandlerFunc
}

func (w *myUpstreamHandlerWrapper) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	myMetadata := w.metadata
	if source.IsValid() {
		myMetadata.Source = source
	}
	if destination.IsValid() {
		myMetadata.Destination = destination
	}
	w.connectionHandler(ctx, conn, myMetadata, onClose)
}

func (w *myUpstreamHandlerWrapper) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	myMetadata := w.metadata
	if source.IsValid() {
		myMetadata.Source = source
	}
	if destination.IsValid() {
		myMetadata.Destination = destination
	}
	w.packetHandler(ctx, conn, myMetadata, onClose)
}

var _ UpstreamHandlerAdapter = (*myUpstreamContextHandlerWrapper)(nil)

type myUpstreamContextHandlerWrapper struct {
	connectionHandler ConnectionHandlerFunc
	packetHandler     PacketConnectionHandlerFunc
}

func NewUpstreamContextHandler(
	connectionHandler ConnectionHandlerFunc,
	packetHandler PacketConnectionHandlerFunc,
) UpstreamHandlerAdapter {
	return &myUpstreamContextHandlerWrapper{
		connectionHandler: connectionHandler,
		packetHandler:     packetHandler,
	}
}

func (w *myUpstreamContextHandlerWrapper) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	_, myMetadata := ExtendContext(ctx)
	if source.IsValid() {
		myMetadata.Source = source
	}
	if destination.IsValid() {
		myMetadata.Destination = destination
	}
	w.connectionHandler(ctx, conn, *myMetadata, onClose)
}

func (w *myUpstreamContextHandlerWrapper) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	_, myMetadata := ExtendContext(ctx)
	if source.IsValid() {
		myMetadata.Source = source
	}
	if destination.IsValid() {
		myMetadata.Destination = destination
	}
	w.packetHandler(ctx, conn, *myMetadata, onClose)
}

func NewRouteHandler(
	metadata InboundContext,
	router ConnectionRouterEx,
) UpstreamHandlerAdapter {
	return &routeHandlerWrapper{
		metadata: metadata,
		router:   router,
	}
}

var _ UpstreamHandlerAdapter = (*routeHandlerWrapper)(nil)

type routeHandlerWrapper struct {
	metadata InboundContext
	router   ConnectionRouterEx
}

func (r *routeHandlerWrapper) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	if source.IsValid() {
		r.metadata.Source = source
	}
	if destination.IsValid() {
		r.metadata.Destination = destination
	}
	r.router.RouteConnectionEx(ctx, conn, r.metadata, onClose)
}

func (r *routeHandlerWrapper) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	if source.IsValid() {
		r.metadata.Source = source
	}
	if destination.IsValid() {
		r.metadata.Destination = destination
	}
	r.router.RoutePacketConnectionEx(ctx, conn, r.metadata, onClose)
}

func NewRouteContextHandler(
	router ConnectionRouterEx,
) UpstreamHandlerAdapter {
	return &routeContextHandlerWrapper{
		router: router,
	}
}

var _ UpstreamHandlerAdapter = (*routeContextHandlerWrapper)(nil)

type routeContextHandlerWrapper struct {
	router ConnectionRouterEx
}

func (r *routeContextHandlerWrapper) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	_, metadata := ExtendContext(ctx)
	if source.IsValid() {
		metadata.Source = source
	}
	if destination.IsValid() {
		metadata.Destination = destination
	}
	r.router.RouteConnectionEx(ctx, conn, *metadata, onClose)
}

func (r *routeContextHandlerWrapper) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	_, metadata := ExtendContext(ctx)
	if source.IsValid() {
		metadata.Source = source
	}
	if destination.IsValid() {
		metadata.Destination = destination
	}
	r.router.RoutePacketConnectionEx(ctx, conn, *metadata, onClose)
}
