package inbound

import (
	"context"
	"net"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	F "github.com/sagernet/sing/common/format"
	N "github.com/sagernet/sing/common/network"

	"github.com/sagernet/sing-shadowsocks"
	"github.com/sagernet/sing-shadowsocks/shadowaead_2022"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

var _ adapter.Inbound = (*ShadowsocksMulti)(nil)

type ShadowsocksMulti struct {
	myInboundAdapter
	service *shadowaead_2022.MultiService[int]
	users   []option.ShadowsocksUser
}

func newShadowsocksMulti(ctx context.Context, router adapter.Router, logger log.Logger, tag string, options option.ShadowsocksInboundOptions) (*ShadowsocksMulti, error) {
	inbound := &ShadowsocksMulti{
		myInboundAdapter: myInboundAdapter{
			protocol:      C.TypeShadowsocks,
			network:       options.Network.Build(),
			ctx:           ctx,
			router:        router,
			logger:        logger,
			tag:           tag,
			listenOptions: options.ListenOptions,
		},
		users: options.Users,
	}
	inbound.connHandler = inbound
	inbound.packetHandler = inbound
	var udpTimeout int64
	if options.UDPTimeout != 0 {
		udpTimeout = options.UDPTimeout
	} else {
		udpTimeout = 300
	}
	service, err := shadowaead_2022.NewMultiServiceWithPassword[int](
		options.Method,
		options.Password,
		udpTimeout,
		adapter.NewUpstreamContextHandler(inbound.newConnection, inbound.newPacketConnection, inbound),
	)
	if err != nil {
		return nil, err
	}
	err = service.UpdateUsersWithPasswords(common.MapIndexed(options.Users, func(index int, user option.ShadowsocksUser) int {
		return index
	}), common.Map(options.Users, func(user option.ShadowsocksUser) string {
		return user.Password
	}))
	if err != nil {
		return nil, err
	}
	inbound.service = service
	inbound.packetUpstream = service
	return inbound, err
}

func (h *ShadowsocksMulti) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return h.service.NewConnection(adapter.ContextWithMetadata(log.ContextWithID(ctx), metadata), conn, adapter.UpstreamMetadata(metadata))
}

func (h *ShadowsocksMulti) NewPacket(ctx context.Context, conn N.PacketConn, buffer *buf.Buffer, metadata adapter.InboundContext) error {
	return h.service.NewPacket(adapter.ContextWithMetadata(log.ContextWithID(ctx), metadata), conn, buffer, adapter.UpstreamMetadata(metadata))
}

func (h *ShadowsocksMulti) newConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	userCtx := ctx.(*shadowsocks.UserContext[int])
	user := h.users[userCtx.User].Name
	if user == "" {
		user = F.ToString(userCtx.User)
	}
	h.logger.WithContext(ctx).Info("[", user, "] inbound connection to ", metadata.Destination)
	return h.router.RouteConnection(ctx, conn, metadata)
}

func (h *ShadowsocksMulti) newPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	userCtx := ctx.(*shadowsocks.UserContext[int])
	user := h.users[userCtx.User].Name
	if user == "" {
		user = F.ToString(userCtx.User)
	}
	ctx = log.ContextWithID(ctx)
	h.logger.WithContext(ctx).Info("[", user, "] inbound packet connection from ", metadata.Source)
	h.logger.WithContext(ctx).Info("[", user, "] inbound packet connection to ", metadata.Destination)
	return h.router.RoutePacketConnection(ctx, conn, metadata)
}
