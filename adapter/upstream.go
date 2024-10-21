package adapter

import (
	"context"
	"net"

	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type (
	ConnectionHandlerFuncEx       = func(ctx context.Context, conn net.Conn, metadata InboundContext, onClose N.CloseHandlerFunc)
	PacketConnectionHandlerFuncEx = func(ctx context.Context, conn N.PacketConn, metadata InboundContext, onClose N.CloseHandlerFunc)
)

func NewUpstreamHandlerEx(
	metadata InboundContext,
	connectionHandler ConnectionHandlerFuncEx,
	packetHandler PacketConnectionHandlerFuncEx,
) UpstreamHandlerAdapterEx {
	return &myUpstreamHandlerWrapperEx{
		metadata:          metadata,
		connectionHandler: connectionHandler,
		packetHandler:     packetHandler,
	}
}

var _ UpstreamHandlerAdapterEx = (*myUpstreamHandlerWrapperEx)(nil)

type myUpstreamHandlerWrapperEx struct {
	metadata          InboundContext
	connectionHandler ConnectionHandlerFuncEx
	packetHandler     PacketConnectionHandlerFuncEx
}

func (w *myUpstreamHandlerWrapperEx) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	myMetadata := w.metadata
	if source.IsValid() {
		myMetadata.Source = source
	}
	if destination.IsValid() {
		myMetadata.Destination = destination
	}
	w.connectionHandler(ctx, conn, myMetadata, onClose)
}

func (w *myUpstreamHandlerWrapperEx) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	myMetadata := w.metadata
	if source.IsValid() {
		myMetadata.Source = source
	}
	if destination.IsValid() {
		myMetadata.Destination = destination
	}
	w.packetHandler(ctx, conn, myMetadata, onClose)
}

var _ UpstreamHandlerAdapterEx = (*myUpstreamContextHandlerWrapperEx)(nil)

type myUpstreamContextHandlerWrapperEx struct {
	connectionHandler ConnectionHandlerFuncEx
	packetHandler     PacketConnectionHandlerFuncEx
}

func NewUpstreamContextHandlerEx(
	connectionHandler ConnectionHandlerFuncEx,
	packetHandler PacketConnectionHandlerFuncEx,
) UpstreamHandlerAdapterEx {
	return &myUpstreamContextHandlerWrapperEx{
		connectionHandler: connectionHandler,
		packetHandler:     packetHandler,
	}
}

func (w *myUpstreamContextHandlerWrapperEx) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	myMetadata := ContextFrom(ctx)
	if source.IsValid() {
		myMetadata.Source = source
	}
	if destination.IsValid() {
		myMetadata.Destination = destination
	}
	w.connectionHandler(ctx, conn, *myMetadata, onClose)
}

func (w *myUpstreamContextHandlerWrapperEx) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	myMetadata := ContextFrom(ctx)
	if source.IsValid() {
		myMetadata.Source = source
	}
	if destination.IsValid() {
		myMetadata.Destination = destination
	}
	w.packetHandler(ctx, conn, *myMetadata, onClose)
}

func NewRouteHandlerEx(
	metadata InboundContext,
	router ConnectionRouterEx,
) UpstreamHandlerAdapterEx {
	return &routeHandlerWrapperEx{
		metadata: metadata,
		router:   router,
	}
}

var _ UpstreamHandlerAdapterEx = (*routeHandlerWrapperEx)(nil)

type routeHandlerWrapperEx struct {
	metadata InboundContext
	router   ConnectionRouterEx
}

func (r *routeHandlerWrapperEx) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	if source.IsValid() {
		r.metadata.Source = source
	}
	if destination.IsValid() {
		r.metadata.Destination = destination
	}
	r.router.RouteConnectionEx(ctx, conn, r.metadata, onClose)
}

func (r *routeHandlerWrapperEx) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	if source.IsValid() {
		r.metadata.Source = source
	}
	if destination.IsValid() {
		r.metadata.Destination = destination
	}
	r.router.RoutePacketConnectionEx(ctx, conn, r.metadata, onClose)
}

func NewRouteContextHandlerEx(
	router ConnectionRouterEx,
) UpstreamHandlerAdapterEx {
	return &routeContextHandlerWrapperEx{
		router: router,
	}
}

var _ UpstreamHandlerAdapterEx = (*routeContextHandlerWrapperEx)(nil)

type routeContextHandlerWrapperEx struct {
	router ConnectionRouterEx
}

func (r *routeContextHandlerWrapperEx) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	metadata := ContextFrom(ctx)
	if source.IsValid() {
		metadata.Source = source
	}
	if destination.IsValid() {
		metadata.Destination = destination
	}
	r.router.RouteConnectionEx(ctx, conn, *metadata, onClose)
}

func (r *routeContextHandlerWrapperEx) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	metadata := ContextFrom(ctx)
	if source.IsValid() {
		metadata.Source = source
	}
	if destination.IsValid() {
		metadata.Destination = destination
	}
	r.router.RoutePacketConnectionEx(ctx, conn, *metadata, onClose)
}
