package inbound

import (
	"context"
	"net"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/mux"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/trojan"
	"github.com/sagernet/sing-box/transport/v2ray"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var (
	_ adapter.Inbound              = (*Trojan)(nil)
	_ adapter.TCPInjectableInbound = (*Trojan)(nil)
)

type Trojan struct {
	myInboundAdapter
	service                  *trojan.Service[int]
	users                    []option.TrojanUser
	tlsConfig                tls.ServerConfig
	fallbackAddr             M.Socksaddr
	fallbackAddrTLSNextProto map[string]M.Socksaddr
	transport                adapter.V2RayServerTransport
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
	if options.TLS != nil {
		tlsConfig, err := tls.NewServer(ctx, logger, common.PtrValueOrDefault(options.TLS))
		if err != nil {
			return nil, err
		}
		inbound.tlsConfig = tlsConfig
	}
	var fallbackHandler N.TCPConnectionHandler
	if options.Fallback != nil && options.Fallback.Server != "" || len(options.FallbackForALPN) > 0 {
		if options.Fallback != nil && options.Fallback.Server != "" {
			inbound.fallbackAddr = options.Fallback.Build()
			if !inbound.fallbackAddr.IsValid() {
				return nil, E.New("invalid fallback address: ", inbound.fallbackAddr)
			}
		}
		if len(options.FallbackForALPN) > 0 {
			if inbound.tlsConfig == nil {
				return nil, E.New("fallback for ALPN is not supported without TLS")
			}
			fallbackAddrNextProto := make(map[string]M.Socksaddr)
			for nextProto, destination := range options.FallbackForALPN {
				fallbackAddr := destination.Build()
				if !fallbackAddr.IsValid() {
					return nil, E.New("invalid fallback address for ALPN ", nextProto, ": ", fallbackAddr)
				}
				fallbackAddrNextProto[nextProto] = fallbackAddr
			}
			inbound.fallbackAddrTLSNextProto = fallbackAddrNextProto
		}
		fallbackHandler = adapter.NewUpstreamContextHandler(inbound.fallbackConnection, nil, nil)
	}
	service := trojan.NewService[int](adapter.NewUpstreamContextHandler(inbound.newConnection, inbound.newPacketConnection, inbound), fallbackHandler)
	err := service.UpdateUsers(common.MapIndexed(options.Users, func(index int, it option.TrojanUser) int {
		return index
	}), common.Map(options.Users, func(it option.TrojanUser) string {
		return it.Password
	}))
	if err != nil {
		return nil, err
	}
	if options.Transport != nil {
		inbound.transport, err = v2ray.NewServerTransport(ctx, logger, common.PtrValueOrDefault(options.Transport), inbound.tlsConfig, (*trojanTransportHandler)(inbound))
		if err != nil {
			return nil, E.Cause(err, "create server transport: ", options.Transport.Type)
		}
	}
	inbound.router, err = mux.NewRouterWithOptions(inbound.router, logger, common.PtrValueOrDefault(options.Multiplex))
	if err != nil {
		return nil, err
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

func (h *Trojan) Close() error {
	return common.Close(
		&h.myInboundAdapter,
		h.tlsConfig,
		h.transport,
	)
}

func (h *Trojan) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	var err error
	if h.tlsConfig != nil && h.transport == nil {
		conn, err = tls.ServerHandshake(ctx, conn, h.tlsConfig)
		if err != nil {
			return err
		}
	}
	return h.service.NewConnection(adapter.WithContext(ctx, &metadata), conn, adapter.UpstreamMetadata(metadata))
}

func (h *Trojan) NewConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	err := h.NewConnection(ctx, conn, metadata)
	N.CloseOnHandshakeFailure(conn, onClose, err)
	if err != nil {
		h.logger.ErrorContext(ctx, E.Cause(err, "process connection from ", metadata.Source))
	}
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

func (h *Trojan) fallbackConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	var fallbackAddr M.Socksaddr
	if len(h.fallbackAddrTLSNextProto) > 0 {
		if tlsConn, loaded := common.Cast[tls.Conn](conn); loaded {
			connectionState := tlsConn.ConnectionState()
			if connectionState.NegotiatedProtocol != "" {
				if fallbackAddr, loaded = h.fallbackAddrTLSNextProto[connectionState.NegotiatedProtocol]; !loaded {
					return E.New("fallback disabled for ALPN: ", connectionState.NegotiatedProtocol)
				}
			}
		}
	}
	if !fallbackAddr.IsValid() {
		if !h.fallbackAddr.IsValid() {
			return E.New("fallback disabled by default")
		}
		fallbackAddr = h.fallbackAddr
	}
	h.logger.InfoContext(ctx, "fallback connection to ", fallbackAddr)
	metadata.Destination = fallbackAddr
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

var _ adapter.V2RayServerTransportHandler = (*trojanTransportHandler)(nil)

type trojanTransportHandler Trojan

func (t *trojanTransportHandler) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	(*Trojan)(t).routeTCP(ctx, conn, source, destination, onClose)
}
