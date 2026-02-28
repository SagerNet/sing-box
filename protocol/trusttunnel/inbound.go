package trusttunnel

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/common/listener"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/xchacha20-poly1305/sing-trusttunnel"
)

func RegistryInbound(registry *inbound.Registry) {
	inbound.Register[option.TrustTunnelInboundOptions](registry, C.TypeTrustTunnel, NewInbound)
}

type Inbound struct {
	inbound.Adapter
	ctx       context.Context
	logger    log.ContextLogger
	router    adapter.ConnectionRouterEx
	listener  *listener.Listener
	service   *trusttunnel.Service
	tlsConfig tls.ServerConfig
	network   []string
}

func NewInbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.TrustTunnelInboundOptions) (adapter.Inbound, error) {
	network := options.Network.Build()
	if common.Contains(network, N.NetworkUDP) {
		if options.TLS == nil || !options.TLS.Enabled {
			return nil, C.ErrTLSRequired
		}
	}
	if len(options.Users) == 0 {
		return nil, E.New("missing users")
	}
	if invalidIndex := common.Index(options.Users, func(it auth.User) bool {
		return it.Username == "" || it.Password == ""
	}); invalidIndex >= 0 {
		return nil, E.New("missing username or password of user ", invalidIndex)
	}
	inbound := &Inbound{
		Adapter: inbound.NewAdapter(C.TypeTrustTunnel, tag),
		ctx:     ctx,
		logger:  logger,
		router:  router,
		listener: listener.New(listener.Options{
			Context: ctx,
			Logger:  logger,
			Listen:  options.ListenOptions,
		}),
		network: network,
	}
	inbound.service = trusttunnel.NewService(trusttunnel.ServiceOptions{
		Ctx:                   ctx,
		Logger:                logger,
		Handler:               inbound,
		ICMPHandler:           nil,
		QUICCongestionControl: options.QUICCongestionControl,
	})
	if options.TLS != nil {
		tlsConfig, err := tls.NewServer(ctx, logger, common.PtrValueOrDefault(options.TLS))
		if err != nil {
			return nil, err
		}
		inbound.tlsConfig = tlsConfig
	}
	inbound.service.UpdateUsers(options.Users)
	return inbound, nil
}

func (h *Inbound) Start(stage adapter.StartStage) (err error) {
	if stage != adapter.StartStateStart {
		return
	}
	if h.tlsConfig != nil {
		err = h.tlsConfig.Start()
		if err != nil {
			err = E.Cause(err, "start TLS config")
			return
		}
	}
	var (
		tcpListener net.Listener
		udpConn     net.PacketConn
	)
	if common.Contains(h.network, N.NetworkTCP) {
		tcpListener, err = h.listener.ListenTCP()
		if err != nil {
			_ = common.Close(h.listener)
			return
		}
	}
	if common.Contains(h.network, N.NetworkUDP) {
		udpConn, err = h.listener.ListenUDP()
		if err != nil {
			_ = common.Close(h.tlsConfig, tcpListener)
			return
		}
	}
	err = h.service.Start(tcpListener, udpConn, h.tlsConfig)
	if err != nil {
		_ = common.Close(h.tlsConfig, tcpListener, udpConn)
		return
	}
	return
}

func (h *Inbound) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	ctx = log.ContextWithNewID(ctx)
	username, _ := auth.UserFromContext[string](ctx)
	metadata := adapter.InboundContext{
		Inbound:     h.Tag(),
		InboundType: h.Type(),
		//nolint:staticcheck
		InboundDetour: h.listener.ListenOptions().Detour,
		//nolint:staticcheck
		InboundOptions:    h.listener.ListenOptions().InboundOptions,
		OriginDestination: h.listener.UDPAddr(),
		Source:            source,
		Destination:       destination,
		User:              username,
	}
	h.router.RouteConnectionEx(ctx, conn, metadata, onClose)
}

func (h *Inbound) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	ctx = log.ContextWithNewID(ctx)
	username, _ := auth.UserFromContext[string](ctx)
	metadata := adapter.InboundContext{
		Inbound:     h.Tag(),
		InboundType: h.Type(),
		//nolint:staticcheck
		InboundDetour: h.listener.ListenOptions().Detour,
		//nolint:staticcheck
		InboundOptions:    h.listener.ListenOptions().InboundOptions,
		OriginDestination: h.listener.UDPAddr(),
		Source:            source,
		Destination:       destination,
		User:              username,
	}
	h.router.RoutePacketConnectionEx(ctx, conn, metadata, onClose)
}

func (h *Inbound) Close() error {
	return common.Close(
		h.service,
		h.tlsConfig,
	)
}
