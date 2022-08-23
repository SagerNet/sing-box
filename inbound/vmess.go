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
	"github.com/sagernet/sing-box/transport/v2ray"
	"github.com/sagernet/sing-vmess"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.Inbound = (*VMess)(nil)

type VMess struct {
	myInboundAdapter
	ctx       context.Context
	service   *vmess.Service[int]
	users     []option.VMessUser
	tlsConfig *TLSConfig
	transport adapter.V2RayServerTransport
}

func NewVMess(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.VMessInboundOptions) (*VMess, error) {
	inbound := &VMess{
		myInboundAdapter: myInboundAdapter{
			protocol:      C.TypeVMess,
			network:       []string{N.NetworkTCP},
			ctx:           ctx,
			router:        router,
			logger:        logger,
			tag:           tag,
			listenOptions: options.ListenOptions,
		},
		ctx:   ctx,
		users: options.Users,
	}
	service := vmess.NewService[int](adapter.NewUpstreamContextHandler(inbound.newConnection, inbound.newPacketConnection, inbound))
	inbound.service = service
	err := service.UpdateUsers(common.MapIndexed(options.Users, func(index int, it option.VMessUser) int {
		return index
	}), common.Map(options.Users, func(it option.VMessUser) string {
		return it.UUID
	}), common.Map(options.Users, func(it option.VMessUser) int {
		return it.AlterId
	}))
	if err != nil {
		return nil, err
	}
	if options.TLS != nil {
		inbound.tlsConfig, err = NewTLSConfig(ctx, logger, common.PtrValueOrDefault(options.TLS))
		if err != nil {
			return nil, err
		}
	}
	if options.Transport != nil {
		var tlsConfig *tls.Config
		if inbound.tlsConfig != nil {
			tlsConfig = inbound.tlsConfig.Config()
		}
		inbound.transport, err = v2ray.NewServerTransport(ctx, common.PtrValueOrDefault(options.Transport), tlsConfig, adapter.NewUpstreamHandler(adapter.InboundContext{}, inbound.newTransportConnection, nil, nil), inbound)
		if err != nil {
			return nil, E.Cause(err, "create server transport: ", options.Transport.Type)
		}
	}
	inbound.connHandler = inbound
	return inbound, nil
}

func (h *VMess) Start() error {
	err := common.Start(
		h.service,
		common.PtrOrNil(h.tlsConfig),
	)
	if err != nil {
		return err
	}
	if h.transport == nil {
		return h.myInboundAdapter.Start()
	}
	if common.Contains(h.transport.Network(), N.NetworkTCP) {
		tcpListener, err := h.myInboundAdapter.ListenTCP()
		if err != nil {
			return err
		}
		go func() {
			sErr := h.transport.Serve(tcpListener)
			if sErr != nil && !E.IsClosed(sErr) {
				h.logger.Error("transport serve error: ", sErr)
			}
		}()
	}
	if common.Contains(h.transport.Network(), N.NetworkUDP) {
		udpConn, err := h.myInboundAdapter.ListenUDP()
		if err != nil {
			return err
		}
		go func() {
			sErr := h.transport.ServePacket(udpConn)
			if sErr != nil && !E.IsClosed(sErr) {
				h.logger.Error("transport serve error: ", sErr)
			}
		}()
	}
	return nil
}

func (h *VMess) Close() error {
	return common.Close(
		h.service,
		&h.myInboundAdapter,
		common.PtrOrNil(h.tlsConfig),
		h.transport,
	)
}

func (h *VMess) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	if h.tlsConfig != nil {
		conn = tls.Server(conn, h.tlsConfig.Config())
	}
	return h.service.NewConnection(adapter.WithContext(log.ContextWithNewID(ctx), &metadata), conn, adapter.UpstreamMetadata(metadata))
}

func (h *VMess) newTransportConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	metadata = h.createMetadata(conn, metadata)
	return h.service.NewConnection(adapter.WithContext(log.ContextWithNewID(ctx), &metadata), conn, adapter.UpstreamMetadata(metadata))
}

func (h *VMess) newConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
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

func (h *VMess) newPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
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
