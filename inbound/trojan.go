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
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/trojan"
)

var _ adapter.Inbound = (*Trojan)(nil)

type Trojan struct {
	myInboundAdapter
	service                  *trojan.Service[int]
	users                    []option.TrojanUser
	tlsConfig                *TLSConfig
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
		tlsConfig, err := NewTLSConfig(ctx, logger, common.PtrValueOrDefault(options.TLS))
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
		var tlsConfig *tls.Config
		if inbound.tlsConfig != nil {
			tlsConfig = inbound.tlsConfig.Config()
		}
		inbound.transport, err = v2ray.NewServerTransport(ctx, common.PtrValueOrDefault(options.Transport), tlsConfig, adapter.NewUpstreamHandler(adapter.InboundContext{}, inbound.newTransportConnection, nil, nil), inbound)
		if err != nil {
			return nil, E.Cause(err, "create server transport: ", options.Transport.Type)
		}
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
		common.PtrOrNil(h.tlsConfig),
		h.transport,
	)
}

func (h *Trojan) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	if h.tlsConfig != nil {
		conn = tls.Server(conn, h.tlsConfig.Config())
	}
	return h.service.NewConnection(adapter.WithContext(log.ContextWithNewID(ctx), &metadata), conn, adapter.UpstreamMetadata(metadata))
}

func (h *Trojan) newTransportConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	metadata = h.createMetadata(conn, metadata)
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

func (h *Trojan) fallbackConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	var fallbackAddr M.Socksaddr
	if len(h.fallbackAddrTLSNextProto) > 0 {
		if tlsConn, loaded := common.Cast[*tls.Conn](conn); loaded {
			connectionState := tlsConn.ConnectionState()
			if connectionState.NegotiatedProtocol != "" {
				if fallbackAddr, loaded = h.fallbackAddrTLSNextProto[connectionState.NegotiatedProtocol]; !loaded {
					return E.New("fallback disabled for ALPN: ", connectionState.NegotiatedProtocol)
				}
			}
		}
	}
	if !fallbackAddr.IsValid() {
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
