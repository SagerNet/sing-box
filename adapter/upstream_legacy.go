package adapter

import (
	"context"
	"net"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type (
	// Deprecated
	ConnectionHandlerFunc = func(ctx context.Context, conn net.Conn, metadata InboundContext) error
	// Deprecated
	PacketConnectionHandlerFunc = func(ctx context.Context, conn N.PacketConn, metadata InboundContext) error
)

// Deprecated
//
//nolint:staticcheck
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

// Deprecated: use myUpstreamHandlerWrapperEx instead.
//
//nolint:staticcheck
type myUpstreamHandlerWrapper struct {
	metadata          InboundContext
	connectionHandler ConnectionHandlerFunc
	packetHandler     PacketConnectionHandlerFunc
	errorHandler      E.Handler
}

// Deprecated: use myUpstreamHandlerWrapperEx instead.
func (w *myUpstreamHandlerWrapper) NewConnection(ctx context.Context, conn net.Conn, metadata M.Metadata) error {
	myMetadata := w.metadata
	if metadata.Source.IsValid() {
		myMetadata.Source = metadata.Source
	}
	if metadata.Destination.IsValid() {
		myMetadata.Destination = metadata.Destination
	}
	return w.connectionHandler(ctx, conn, myMetadata)
}

// Deprecated: use myUpstreamHandlerWrapperEx instead.
func (w *myUpstreamHandlerWrapper) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata M.Metadata) error {
	myMetadata := w.metadata
	if metadata.Source.IsValid() {
		myMetadata.Source = metadata.Source
	}
	if metadata.Destination.IsValid() {
		myMetadata.Destination = metadata.Destination
	}
	return w.packetHandler(ctx, conn, myMetadata)
}

// Deprecated: use myUpstreamHandlerWrapperEx instead.
func (w *myUpstreamHandlerWrapper) NewError(ctx context.Context, err error) {
	w.errorHandler.NewError(ctx, err)
}

// Deprecated: removed
func UpstreamMetadata(metadata InboundContext) M.Metadata {
	return M.Metadata{
		Source:      metadata.Source.Unwrap(),
		Destination: metadata.Destination.Unwrap(),
	}
}

// Deprecated: Use NewUpstreamContextHandlerEx instead.
type myUpstreamContextHandlerWrapper struct {
	connectionHandler ConnectionHandlerFunc
	packetHandler     PacketConnectionHandlerFunc
	errorHandler      E.Handler
}

// Deprecated: Use NewUpstreamContextHandlerEx instead.
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

// Deprecated: Use NewUpstreamContextHandlerEx instead.
func (w *myUpstreamContextHandlerWrapper) NewConnection(ctx context.Context, conn net.Conn, metadata M.Metadata) error {
	myMetadata := ContextFrom(ctx)
	if metadata.Source.IsValid() {
		myMetadata.Source = metadata.Source
	}
	if metadata.Destination.IsValid() {
		myMetadata.Destination = metadata.Destination
	}
	return w.connectionHandler(ctx, conn, *myMetadata)
}

// Deprecated: Use NewUpstreamContextHandlerEx instead.
func (w *myUpstreamContextHandlerWrapper) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata M.Metadata) error {
	myMetadata := ContextFrom(ctx)
	if metadata.Source.IsValid() {
		myMetadata.Source = metadata.Source
	}
	if metadata.Destination.IsValid() {
		myMetadata.Destination = metadata.Destination
	}
	return w.packetHandler(ctx, conn, *myMetadata)
}

// Deprecated: Use NewUpstreamContextHandlerEx instead.
func (w *myUpstreamContextHandlerWrapper) NewError(ctx context.Context, err error) {
	w.errorHandler.NewError(ctx, err)
}

// Deprecated: Use ConnectionRouterEx instead.
func NewRouteHandler(
	metadata InboundContext,
	router ConnectionRouter,
	logger logger.ContextLogger,
) UpstreamHandlerAdapter {
	return &routeHandlerWrapper{
		metadata: metadata,
		router:   router,
		logger:   logger,
	}
}

// Deprecated: Use ConnectionRouterEx instead.
func NewRouteContextHandler(
	router ConnectionRouter,
	logger logger.ContextLogger,
) UpstreamHandlerAdapter {
	return &routeContextHandlerWrapper{
		router: router,
		logger: logger,
	}
}

var _ UpstreamHandlerAdapter = (*routeHandlerWrapper)(nil)

// Deprecated: Use ConnectionRouterEx instead.
//
//nolint:staticcheck
type routeHandlerWrapper struct {
	metadata InboundContext
	router   ConnectionRouter
	logger   logger.ContextLogger
}

// Deprecated: Use ConnectionRouterEx instead.
func (w *routeHandlerWrapper) NewConnection(ctx context.Context, conn net.Conn, metadata M.Metadata) error {
	myMetadata := w.metadata
	if metadata.Source.IsValid() {
		myMetadata.Source = metadata.Source
	}
	if metadata.Destination.IsValid() {
		myMetadata.Destination = metadata.Destination
	}
	return w.router.RouteConnection(ctx, conn, myMetadata)
}

// Deprecated: Use ConnectionRouterEx instead.
func (w *routeHandlerWrapper) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata M.Metadata) error {
	myMetadata := w.metadata
	if metadata.Source.IsValid() {
		myMetadata.Source = metadata.Source
	}
	if metadata.Destination.IsValid() {
		myMetadata.Destination = metadata.Destination
	}
	return w.router.RoutePacketConnection(ctx, conn, myMetadata)
}

// Deprecated: Use ConnectionRouterEx instead.
func (w *routeHandlerWrapper) NewError(ctx context.Context, err error) {
	w.logger.ErrorContext(ctx, err)
}

var _ UpstreamHandlerAdapter = (*routeContextHandlerWrapper)(nil)

// Deprecated: Use ConnectionRouterEx instead.
type routeContextHandlerWrapper struct {
	router ConnectionRouter
	logger logger.ContextLogger
}

// Deprecated: Use ConnectionRouterEx instead.
func (w *routeContextHandlerWrapper) NewConnection(ctx context.Context, conn net.Conn, metadata M.Metadata) error {
	myMetadata := ContextFrom(ctx)
	if metadata.Source.IsValid() {
		myMetadata.Source = metadata.Source
	}
	if metadata.Destination.IsValid() {
		myMetadata.Destination = metadata.Destination
	}
	return w.router.RouteConnection(ctx, conn, *myMetadata)
}

// Deprecated: Use ConnectionRouterEx instead.
func (w *routeContextHandlerWrapper) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata M.Metadata) error {
	myMetadata := ContextFrom(ctx)
	if metadata.Source.IsValid() {
		myMetadata.Source = metadata.Source
	}
	if metadata.Destination.IsValid() {
		myMetadata.Destination = metadata.Destination
	}
	return w.router.RoutePacketConnection(ctx, conn, *myMetadata)
}

// Deprecated: Use ConnectionRouterEx instead.
func (w *routeContextHandlerWrapper) NewError(ctx context.Context, err error) {
	w.logger.ErrorContext(ctx, err)
}
