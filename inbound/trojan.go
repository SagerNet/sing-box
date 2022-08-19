package inbound

import (
	"context"
	"crypto/tls"
	"net"
	"os"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/trojan"
)

var _ adapter.Inbound = (*Trojan)(nil)

type Trojan struct {
	myInboundAdapter
	service   *trojan.Service[int]
	users     []option.TrojanUser
	tlsConfig *TLSConfig
}

func NewTrojan(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.TrojanInboundOptions) (*Trojan, error) {
	inbound := &Trojan{
		myInboundAdapter: myInboundAdapter{
			protocol:      C.TypeTrojan,
			network:       []string{N.NetworkTCP},
			ctx:           ctx,
			router:        router,
			logger:        logger,
			tag:           tag,
			listenOptions: options.ListenOptions,
		},
		users: options.Users,
	}
	service := trojan.NewService[int](adapter.NewUpstreamContextHandler(inbound.newConnection, inbound.newPacketConnection, inbound))
	err := service.UpdateUsers(common.MapIndexed(options.Users, func(index int, it option.TrojanUser) int {
		return index
	}), common.Map(options.Users, func(it option.TrojanUser) string {
		return it.Password
	}))
	if err != nil {
		return nil, err
	}
	if options.TLS != nil {
		tlsConfig, err := NewTLSConfig(ctx, logger, common.PtrValueOrDefault(options.TLS))
		if err != nil {
			return nil, err
		}
		inbound.tlsConfig = tlsConfig
	}
	inbound.service = service
	inbound.connHandler = inbound
	return inbound, nil
}

func (h *Trojan) Start() error {
	if h.tlsConfig != nil {
		err := h.tlsConfig.Start()
		if err != nil {
			return E.Cause(err, "create TLS config")
		}
	}
	return common.Start(
		h.service,
		&h.myInboundAdapter,
	)
}

func (h *Trojan) Close() error {
	return common.Close(
		h.service,
		&h.myInboundAdapter,
		common.PtrOrNil(h.tlsConfig),
	)
}

func (h *Trojan) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	if h.tlsConfig != nil {
		conn = tls.Server(conn, h.tlsConfig.Config())
	}
	return h.service.NewConnection(adapter.WithContext(log.ContextWithNewID(ctx), &metadata), conn, adapter.UpstreamMetadata(metadata))
}

func (h *Trojan) newConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	userIndex, loaded := auth.UserFromContext[int](ctx)
	if !loaded {
		return os.ErrInvalid
	}
	user := h.users[userIndex].Name
	if user == "" {
		user = F.ToString(userIndex)
	} else {
		metadata.User = user
	}
	h.logger.InfoContext(ctx, "[", user, "] inbound connection to ", metadata.Destination)
	return h.router.RouteConnection(ctx, conn, metadata)
}

func (h *Trojan) newPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	userIndex, loaded := auth.UserFromContext[int](ctx)
	if !loaded {
		return os.ErrInvalid
	}
	user := h.users[userIndex].Name
	if user == "" {
		user = F.ToString(userIndex)
	} else {
		metadata.User = user
	}
	h.logger.InfoContext(ctx, "[", user, "] inbound packet connection to ", metadata.Destination)
	return h.router.RoutePacketConnection(ctx, conn, metadata)
}
