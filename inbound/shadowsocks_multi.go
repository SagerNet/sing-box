package inbound

import (
	"context"
	"net"
	"net/http"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/pipelistener"
	"github.com/sagernet/sing-box/common/trafficcontrol"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-shadowsocks/shadowaead_2022"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.Inbound = (*ShadowsocksMulti)(nil)

type ShadowsocksMulti struct {
	myInboundAdapter
	service        *shadowaead_2022.MultiService[int]
	users          []option.ShadowsocksUser
	controlEnabled bool
	controller     *http.Server
	controllerPipe *pipelistener.Listener
	trafficManager *trafficcontrol.Manager[int]
}

func newShadowsocksMulti(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.ShadowsocksInboundOptions) (*ShadowsocksMulti, error) {
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
	}
	inbound.connHandler = inbound
	inbound.packetHandler = inbound
	var udpTimeout int64
	if options.UDPTimeout != 0 {
		udpTimeout = options.UDPTimeout
	} else {
		udpTimeout = int64(C.UDPTimeout.Seconds())
	}
	service, err := shadowaead_2022.NewMultiServiceWithPassword[int](
		options.Method,
		options.Password,
		udpTimeout,
		adapter.NewUpstreamContextHandler(inbound.newConnection, inbound.newPacketConnection, inbound),
	)
	users := options.Users
	if options.ControlPassword != "" {
		inbound.controlEnabled = true
		users = append([]option.ShadowsocksUser{{
			Name:     "control",
			Password: options.ControlPassword,
		}}, users...)
		inbound.controller = &http.Server{Handler: inbound.createHandler()}
		inbound.trafficManager = trafficcontrol.NewManager[int]()
	}
	if err != nil {
		return nil, err
	}
	err = service.UpdateUsersWithPasswords(common.MapIndexed(users, func(index int, user option.ShadowsocksUser) int {
		return index
	}), common.Map(options.Users, func(user option.ShadowsocksUser) string {
		return user.Password
	}))
	if err != nil {
		return nil, err
	}
	inbound.service = service
	inbound.packetUpstream = service
	inbound.users = users
	return inbound, err
}

func (h *ShadowsocksMulti) Start() error {
	if h.controlEnabled {
		h.controllerPipe = pipelistener.New(16)
		go func() {
			err := h.controller.Serve(h.controllerPipe)
			if err != nil {
				h.newError(E.Cause(err, "controller serve error"))
			}
		}()
	}
	return h.myInboundAdapter.Start()
}

func (h *ShadowsocksMulti) Close() error {
	if h.controlEnabled {
		h.controllerPipe.Close()
	}
	return h.myInboundAdapter.Close()
}

func (h *ShadowsocksMulti) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return h.service.NewConnection(adapter.WithContext(log.ContextWithNewID(ctx), &metadata), conn, adapter.UpstreamMetadata(metadata))
}

func (h *ShadowsocksMulti) NewPacket(ctx context.Context, conn N.PacketConn, buffer *buf.Buffer, metadata adapter.InboundContext) error {
	return h.service.NewPacket(adapter.WithContext(ctx, &metadata), conn, buffer, adapter.UpstreamMetadata(metadata))
}

func (h *ShadowsocksMulti) newConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	userIndex, loaded := auth.UserFromContext[int](ctx)
	if !loaded {
		return os.ErrInvalid
	}
	if userIndex == 0 && h.controlEnabled {
		h.logger.InfoContext(ctx, "inbound control connection")
		h.controllerPipe.Serve(conn)
		return nil
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

func (h *ShadowsocksMulti) newPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
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
	ctx = log.ContextWithNewID(ctx)
	h.logger.InfoContext(ctx, "[", user, "] inbound packet connection from ", metadata.Source)
	h.logger.InfoContext(ctx, "[", user, "] inbound packet connection to ", metadata.Destination)
	return h.router.RoutePacketConnection(ctx, conn, metadata)
}
