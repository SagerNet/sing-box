package adapter

import (
	"context"
	"net"

	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type ConnectionRouter interface {
	RouteConnection(ctx context.Context, conn net.Conn, metadata InboundContext) error
	RoutePacketConnection(ctx context.Context, conn N.PacketConn, metadata InboundContext) error
}

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

type routeHandlerWrapper struct {
	metadata InboundContext
	router   ConnectionRouter
	logger   logger.ContextLogger
}

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

func (w *routeHandlerWrapper) NewError(ctx context.Context, err error) {
	w.logger.ErrorContext(ctx, err)
}

var _ UpstreamHandlerAdapter = (*routeContextHandlerWrapper)(nil)

type routeContextHandlerWrapper struct {
	router ConnectionRouter
	logger logger.ContextLogger
}

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

func (w *routeContextHandlerWrapper) NewError(ctx context.Context, err error) {
	w.logger.ErrorContext(ctx, err)
}
