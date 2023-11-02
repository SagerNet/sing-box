package mux

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-mux"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	N "github.com/sagernet/sing/common/network"
)

type Router struct {
	router  adapter.ConnectionRouter
	service *mux.Service
}

func NewRouterWithOptions(router adapter.ConnectionRouter, logger logger.ContextLogger, options option.InboundMultiplexOptions) (adapter.ConnectionRouter, error) {
	if !options.Enabled {
		return router, nil
	}
	var brutalOptions mux.BrutalOptions
	if options.Brutal != nil && options.Brutal.Enabled {
		brutalOptions = mux.BrutalOptions{
			Enabled:    true,
			SendBPS:    uint64(options.Brutal.UpMbps * C.MbpsToBps),
			ReceiveBPS: uint64(options.Brutal.DownMbps * C.MbpsToBps),
		}
		if brutalOptions.SendBPS < mux.BrutalMinSpeedBPS {
			return nil, E.New("brutal: invalid upload speed")
		}
		if brutalOptions.ReceiveBPS < mux.BrutalMinSpeedBPS {
			return nil, E.New("brutal: invalid download speed")
		}
	}
	service, err := mux.NewService(mux.ServiceOptions{
		NewStreamContext: func(ctx context.Context, conn net.Conn) context.Context {
			return log.ContextWithNewID(ctx)
		},
		Logger:  logger,
		Handler: adapter.NewRouteContextHandler(router, logger),
		Padding: options.Padding,
		Brutal:  brutalOptions,
	})
	if err != nil {
		return nil, err
	}
	return &Router{router, service}, nil
}

func (r *Router) RouteConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	if metadata.Destination == mux.Destination {
		return r.service.NewConnection(adapter.WithContext(ctx, &metadata), conn, adapter.UpstreamMetadata(metadata))
	} else {
		return r.router.RouteConnection(ctx, conn, metadata)
	}
}

func (r *Router) RoutePacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return r.router.RoutePacketConnection(ctx, conn, metadata)
}
