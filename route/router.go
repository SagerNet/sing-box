package route

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.Router = (*Router)(nil)

type Router struct {
	logger          log.Logger
	defaultOutbound adapter.Outbound
	outboundByTag   map[string]adapter.Outbound
}

func NewRouter(logger log.Logger) *Router {
	return &Router{
		logger:        logger,
		outboundByTag: make(map[string]adapter.Outbound),
	}
}

func (r *Router) AddOutbound(outbound adapter.Outbound) {
	if outbound.Tag() != "" {
		r.outboundByTag[outbound.Tag()] = outbound
	}
	if r.defaultOutbound == nil {
		r.defaultOutbound = outbound
	}
}

func (r *Router) DefaultOutbound() adapter.Outbound {
	if r.defaultOutbound == nil {
		panic("missing default outbound")
	}
	return r.defaultOutbound
}

func (r *Router) Outbound(tag string) (adapter.Outbound, bool) {
	outbound, loaded := r.outboundByTag[tag]
	return outbound, loaded
}

func (r *Router) RouteConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	r.logger.WithContext(ctx).Debug("no match")
	r.logger.WithContext(ctx).Debug("route connection to default outbound")
	return r.defaultOutbound.NewConnection(ctx, conn, metadata.Destination)
}

func (r *Router) RoutePacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	r.logger.WithContext(ctx).Debug("no match")
	r.logger.WithContext(ctx).Debug("route packet connection to default outbound")
	return r.defaultOutbound.NewPacketConnection(ctx, conn, metadata.Destination)
}
