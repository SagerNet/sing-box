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
	LegacyConnectionHandlerFunc = func(ctx context.Context, conn net.Conn, metadata InboundContext) error
	// Deprecated
	LegacyPacketConnectionHandlerFunc = func(ctx context.Context, conn N.PacketConn, metadata InboundContext) error
)

// Deprecated
//
//nolint:staticcheck
type LegacyUpstreamHandlerAdapter interface {
	N.TCPConnectionHandler
	N.UDPConnectionHandler
	E.Handler
}

// Deprecated
//
//nolint:staticcheck
func NewLegacyUpstreamHandler(
	metadata InboundContext,
	connectionHandler LegacyConnectionHandlerFunc,
	packetHandler LegacyPacketConnectionHandlerFunc,
	errorHandler E.Handler,
) LegacyUpstreamHandlerAdapter {
	return &legacyUpstreamHandlerWrapper{
		metadata:          metadata,
		connectionHandler: connectionHandler,
		packetHandler:     packetHandler,
		errorHandler:      errorHandler,
	}
}

var _ LegacyUpstreamHandlerAdapter = (*legacyUpstreamHandlerWrapper)(nil)

// Deprecated: use NewUpstreamHandler instead.
//
//nolint:staticcheck
type legacyUpstreamHandlerWrapper struct {
	metadata          InboundContext
	connectionHandler LegacyConnectionHandlerFunc
	packetHandler     LegacyPacketConnectionHandlerFunc
	errorHandler      E.Handler
}

// Deprecated: use NewUpstreamHandler instead.
func (w *legacyUpstreamHandlerWrapper) NewConnection(ctx context.Context, conn net.Conn, metadata M.Metadata) error {
	myMetadata := w.metadata
	if metadata.Source.IsValid() {
		myMetadata.Source = metadata.Source
	}
	if metadata.Destination.IsValid() {
		myMetadata.Destination = metadata.Destination
	}
	return w.connectionHandler(ctx, conn, myMetadata)
}

// Deprecated: use NewUpstreamHandler instead.
func (w *legacyUpstreamHandlerWrapper) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata M.Metadata) error {
	myMetadata := w.metadata
	if metadata.Source.IsValid() {
		myMetadata.Source = metadata.Source
	}
	if metadata.Destination.IsValid() {
		myMetadata.Destination = metadata.Destination
	}
	return w.packetHandler(ctx, conn, myMetadata)
}

// Deprecated: use NewUpstreamHandler instead.
func (w *legacyUpstreamHandlerWrapper) NewError(ctx context.Context, err error) {
	w.errorHandler.NewError(ctx, err)
}

// Deprecated: removed
func UpstreamMetadata(metadata InboundContext) M.Metadata {
	return M.Metadata{
		Source:      metadata.Source.Unwrap(),
		Destination: metadata.Destination.Unwrap(),
	}
}

// Deprecated: Use NewUpstreamContextHandler instead.
type legacyUpstreamContextHandlerWrapper struct {
	connectionHandler LegacyConnectionHandlerFunc
	packetHandler     LegacyPacketConnectionHandlerFunc
	errorHandler      E.Handler
}

// Deprecated: Use NewUpstreamContextHandler instead.
func NewLegacyUpstreamContextHandler(
	connectionHandler LegacyConnectionHandlerFunc,
	packetHandler LegacyPacketConnectionHandlerFunc,
	errorHandler E.Handler,
) LegacyUpstreamHandlerAdapter {
	return &legacyUpstreamContextHandlerWrapper{
		connectionHandler: connectionHandler,
		packetHandler:     packetHandler,
		errorHandler:      errorHandler,
	}
}

// Deprecated: Use NewUpstreamContextHandler instead.
func (w *legacyUpstreamContextHandlerWrapper) NewConnection(ctx context.Context, conn net.Conn, metadata M.Metadata) error {
	myMetadata := ContextFrom(ctx)
	if metadata.Source.IsValid() {
		myMetadata.Source = metadata.Source
	}
	if metadata.Destination.IsValid() {
		myMetadata.Destination = metadata.Destination
	}
	return w.connectionHandler(ctx, conn, *myMetadata)
}

// Deprecated: Use NewUpstreamContextHandler instead.
func (w *legacyUpstreamContextHandlerWrapper) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata M.Metadata) error {
	myMetadata := ContextFrom(ctx)
	if metadata.Source.IsValid() {
		myMetadata.Source = metadata.Source
	}
	if metadata.Destination.IsValid() {
		myMetadata.Destination = metadata.Destination
	}
	return w.packetHandler(ctx, conn, *myMetadata)
}

// Deprecated: Use NewUpstreamContextHandler instead.
func (w *legacyUpstreamContextHandlerWrapper) NewError(ctx context.Context, err error) {
	w.errorHandler.NewError(ctx, err)
}

// Deprecated: Use ConnectionRouterEx instead.
func NewLegacyRouteHandler(
	metadata InboundContext,
	router ConnectionRouter,
	logger logger.ContextLogger,
) LegacyUpstreamHandlerAdapter {
	return &legacyRouteHandlerWrapper{
		metadata: metadata,
		router:   router,
		logger:   logger,
	}
}

// Deprecated: Use ConnectionRouterEx instead.
func NewLegacyRouteContextHandler(
	router ConnectionRouter,
	logger logger.ContextLogger,
) LegacyUpstreamHandlerAdapter {
	return &legacyRouteContextHandlerWrapper{
		router: router,
		logger: logger,
	}
}

var _ LegacyUpstreamHandlerAdapter = (*legacyRouteHandlerWrapper)(nil)

// Deprecated: Use ConnectionRouterEx instead.
//
//nolint:staticcheck
type legacyRouteHandlerWrapper struct {
	metadata InboundContext
	router   ConnectionRouter
	logger   logger.ContextLogger
}

// Deprecated: Use ConnectionRouterEx instead.
func (w *legacyRouteHandlerWrapper) NewConnection(ctx context.Context, conn net.Conn, metadata M.Metadata) error {
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
func (w *legacyRouteHandlerWrapper) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata M.Metadata) error {
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
func (w *legacyRouteHandlerWrapper) NewError(ctx context.Context, err error) {
	w.logger.ErrorContext(ctx, err)
}

var _ LegacyUpstreamHandlerAdapter = (*legacyRouteContextHandlerWrapper)(nil)

// Deprecated: Use ConnectionRouterEx instead.
type legacyRouteContextHandlerWrapper struct {
	router ConnectionRouter
	logger logger.ContextLogger
}

// Deprecated: Use ConnectionRouterEx instead.
func (w *legacyRouteContextHandlerWrapper) NewConnection(ctx context.Context, conn net.Conn, metadata M.Metadata) error {
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
func (w *legacyRouteContextHandlerWrapper) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata M.Metadata) error {
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
func (w *legacyRouteContextHandlerWrapper) NewError(ctx context.Context, err error) {
	w.logger.ErrorContext(ctx, err)
}
