package mux

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	vmess "github.com/sagernet/sing-vmess"
	"github.com/sagernet/sing/common/logger"
	N "github.com/sagernet/sing/common/network"
)

type V2RayLegacyRouter struct {
	router adapter.ConnectionRouter
	logger logger.ContextLogger
}

func NewV2RayLegacyRouter(router adapter.ConnectionRouter, logger logger.ContextLogger) adapter.ConnectionRouter {
	return &V2RayLegacyRouter{router, logger}
}

func (r *V2RayLegacyRouter) RouteConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	if metadata.Destination.Fqdn == vmess.MuxDestination.Fqdn {
		r.logger.InfoContext(ctx, "inbound legacy multiplex connection")
		return vmess.HandleMuxConnection(ctx, conn, adapter.NewRouteHandler(metadata, r.router, r.logger))
	}
	return r.router.RouteConnection(ctx, conn, metadata)
}

func (r *V2RayLegacyRouter) RoutePacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return r.router.RoutePacketConnection(ctx, conn, metadata)
}
