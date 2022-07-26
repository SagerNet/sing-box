package inbound

import (
	"context"
	"net"
	"os"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-shadowsocks/shadowaead_2022"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/buf"
	F "github.com/sagernet/sing/common/format"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.Inbound = (*ShadowsocksMulti)(nil)

type ShadowsocksRelay struct {
	myInboundAdapter
	service      *shadowaead_2022.RelayService[int]
	destinations []option.ShadowsocksDestination
}

func newShadowsocksRelay(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.ShadowsocksInboundOptions) (*ShadowsocksRelay, error) {
	inbound := &ShadowsocksRelay{
		myInboundAdapter: myInboundAdapter{
			protocol:      C.TypeShadowsocks,
			network:       options.Network.Build(),
			ctx:           ctx,
			router:        router,
			logger:        logger,
			tag:           tag,
			listenOptions: options.ListenOptions,
		},
		destinations: options.Destinations,
	}
	inbound.connHandler = inbound
	inbound.packetHandler = inbound
	var udpTimeout int64
	if options.UDPTimeout != 0 {
		udpTimeout = options.UDPTimeout
	} else {
		udpTimeout = int64(C.UDPTimeout.Seconds())
	}
	service, err := shadowaead_2022.NewRelayServiceWithPassword[int](
		options.Method,
		options.Password,
		udpTimeout,
		adapter.NewUpstreamContextHandler(inbound.newConnection, inbound.newPacketConnection, inbound),
	)
	if err != nil {
		return nil, err
	}
	err = service.UpdateUsersWithPasswords(common.MapIndexed(options.Destinations, func(index int, user option.ShadowsocksDestination) int {
		return index
	}), common.Map(options.Destinations, func(user option.ShadowsocksDestination) string {
		return user.Password
	}), common.Map(options.Destinations, option.ShadowsocksDestination.Build))
	if err != nil {
		return nil, err
	}
	inbound.service = service
	inbound.packetUpstream = service
	return inbound, err
}

func (h *ShadowsocksRelay) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return h.service.NewConnection(adapter.WithContext(log.ContextWithNewID(ctx), &metadata), conn, adapter.UpstreamMetadata(metadata))
}

func (h *ShadowsocksRelay) NewPacket(ctx context.Context, conn N.PacketConn, buffer *buf.Buffer, metadata adapter.InboundContext) error {
	return h.service.NewPacket(adapter.WithContext(ctx, &metadata), conn, buffer, adapter.UpstreamMetadata(metadata))
}

func (h *ShadowsocksRelay) newConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	destinationIndex, loaded := auth.UserFromContext[int](ctx)
	if !loaded {
		return os.ErrInvalid
	}
	destination := h.destinations[destinationIndex].Name
	if destination == "" {
		destination = F.ToString(destinationIndex)
	} else {
		metadata.User = destination
	}
	h.logger.InfoContext(ctx, "[", destination, "] inbound connection to ", metadata.Destination)
	return h.router.RouteConnection(ctx, conn, metadata)
}

func (h *ShadowsocksRelay) newPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	destinationIndex, loaded := auth.UserFromContext[int](ctx)
	if !loaded {
		return os.ErrInvalid
	}
	destination := h.destinations[destinationIndex].Name
	if destination == "" {
		destination = F.ToString(destinationIndex)
	} else {
		metadata.User = destination
	}
	ctx = log.ContextWithNewID(ctx)
	h.logger.InfoContext(ctx, "[", destination, "] inbound packet connection from ", metadata.Source)
	h.logger.InfoContext(ctx, "[", destination, "] inbound packet connection to ", metadata.Destination)
	return h.router.RoutePacketConnection(ctx, conn, metadata)
}
