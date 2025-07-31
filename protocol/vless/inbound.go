package vless

import (
	"context"
	"net"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/common/listener"
	"github.com/sagernet/sing-box/common/mux"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/common/uot"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/v2ray"
	"github.com/sagernet/sing-vmess/packetaddr"
	"github.com/sagernet/sing-vmess/vless"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func RegisterInbound(registry *inbound.Registry) {
	inbound.Register[option.VLESSInboundOptions](registry, C.TypeVLESS, NewInbound)
}

var _ adapter.TCPInjectableInbound = (*Inbound)(nil)

type Inbound struct {
	inbound.Adapter
	ctx       context.Context
	router    adapter.ConnectionRouterEx
	logger    logger.ContextLogger
	listener  *listener.Listener
	users     []option.VLESSUser
	service   *vless.Service[int]
	tlsConfig tls.ServerConfig
	transport adapter.V2RayServerTransport
}

func NewInbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.VLESSInboundOptions) (adapter.Inbound, error) {
	inbound := &Inbound{
		Adapter: inbound.NewAdapter(C.TypeVLESS, tag),
		ctx:     ctx,
		router:  uot.NewRouter(router, logger),
		logger:  logger,
		users:   options.Users,
	}
	var err error
	inbound.router, err = mux.NewRouterWithOptions(inbound.router, logger, common.PtrValueOrDefault(options.Multiplex))
	if err != nil {
		return nil, err
	}
	service := vless.NewService[int](logger, adapter.NewUpstreamContextHandlerEx(inbound.newConnectionEx, inbound.newPacketConnectionEx))
	service.UpdateUsers(common.MapIndexed(inbound.users, func(index int, _ option.VLESSUser) int {
		return index
	}), common.Map(inbound.users, func(it option.VLESSUser) string {
		return it.UUID
	}), common.Map(inbound.users, func(it option.VLESSUser) string {
		return it.Flow
	}))
	inbound.service = service
	if options.TLS != nil {
		inbound.tlsConfig, err = tls.NewServer(ctx, logger, common.PtrValueOrDefault(options.TLS))
		if err != nil {
			return nil, err
		}
	}
	if options.Transport != nil {
		inbound.transport, err = v2ray.NewServerTransport(ctx, logger, common.PtrValueOrDefault(options.Transport), inbound.tlsConfig, (*inboundTransportHandler)(inbound))
		if err != nil {
			return nil, E.Cause(err, "create server transport: ", options.Transport.Type)
		}
	}
	inbound.listener = listener.New(listener.Options{
		Context:           ctx,
		Logger:            logger,
		Network:           []string{N.NetworkTCP},
		Listen:            options.ListenOptions,
		ConnectionHandler: inbound,
	})
	return inbound, nil
}

func (h *Inbound) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	if h.tlsConfig != nil {
		err := h.tlsConfig.Start()
		if err != nil {
			return err
		}
	}
	if h.transport == nil {
		return h.listener.Start()
	}
	if common.Contains(h.transport.Network(), N.NetworkTCP) {
		tcpListener, err := h.listener.ListenTCP()
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
		udpConn, err := h.listener.ListenUDP()
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

func (h *Inbound) Close() error {
	return common.Close(
		h.service,
		h.listener,
		h.tlsConfig,
		h.transport,
	)
}

func (h *Inbound) NewConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	if h.tlsConfig != nil && h.transport == nil {
		tlsConn, err := tls.ServerHandshake(ctx, conn, h.tlsConfig)
		if err != nil {
			N.CloseOnHandshakeFailure(conn, onClose, err)
			h.logger.ErrorContext(ctx, E.Cause(err, "process connection from ", metadata.Source, ": TLS handshake"))
			return
		}
		conn = tlsConn
	}
	err := h.service.NewConnection(adapter.WithContext(ctx, &metadata), conn, metadata.Source, onClose)
	if err != nil {
		N.CloseOnHandshakeFailure(conn, onClose, err)
		h.logger.ErrorContext(ctx, E.Cause(err, "process connection from ", metadata.Source))
	}
}

func (h *Inbound) newConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	metadata.Inbound = h.Tag()
	metadata.InboundType = h.Type()
	userIndex, loaded := auth.UserFromContext[int](ctx)
	if !loaded {
		N.CloseOnHandshakeFailure(conn, onClose, os.ErrInvalid)
		return
	}
	user := h.users[userIndex].Name
	if user == "" {
		user = F.ToString(userIndex)
	} else {
		metadata.User = user
	}
	h.logger.InfoContext(ctx, "[", user, "] inbound connection to ", metadata.Destination)
	h.router.RouteConnectionEx(ctx, conn, metadata, onClose)
}

func (h *Inbound) newPacketConnectionEx(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	metadata.Inbound = h.Tag()
	metadata.InboundType = h.Type()
	userIndex, loaded := auth.UserFromContext[int](ctx)
	if !loaded {
		N.CloseOnHandshakeFailure(conn, onClose, os.ErrInvalid)
		return
	}
	user := h.users[userIndex].Name
	if user == "" {
		user = F.ToString(userIndex)
	} else {
		metadata.User = user
	}
	if metadata.Destination.Fqdn == packetaddr.SeqPacketMagicAddress {
		metadata.Destination = M.Socksaddr{}
		conn = packetaddr.NewConn(bufio.NewNetPacketConn(conn), metadata.Destination)
		h.logger.InfoContext(ctx, "[", user, "] inbound packet addr connection")
	} else {
		h.logger.InfoContext(ctx, "[", user, "] inbound packet connection to ", metadata.Destination)
	}
	h.router.RoutePacketConnectionEx(ctx, conn, metadata, onClose)
}

var _ adapter.V2RayServerTransportHandler = (*inboundTransportHandler)(nil)

type inboundTransportHandler Inbound

func (h *inboundTransportHandler) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	var metadata adapter.InboundContext
	metadata.Source = source
	metadata.Destination = destination
	//nolint:staticcheck
	metadata.InboundDetour = h.listener.ListenOptions().Detour
	//nolint:staticcheck
	metadata.InboundOptions = h.listener.ListenOptions().InboundOptions
	h.logger.InfoContext(ctx, "inbound connection from ", metadata.Source)
	(*Inbound)(h).NewConnectionEx(ctx, conn, metadata, onClose)
}
