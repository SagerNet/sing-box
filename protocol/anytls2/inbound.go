package anytls2

import (
	"context"
	"net"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/common/listener"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/common/uot"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/v2ray"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	anytls "github.com/anytls/sing-anytls"
	"github.com/anytls/sing-anytls/padding"
)

func RegisterInbound(registry *inbound.Registry) {
	inbound.Register[option.AnyTLS2InboundOptions](registry, C.TypeAnyTLS2, NewInbound)
}

var _ adapter.TCPInjectableInbound = (*Inbound)(nil)
var _ adapter.V2RayServerTransportHandler = (*inboundTransportHandler)(nil)

type Inbound struct {
	inbound.Adapter
	tlsConfig tls.ServerConfig
	router    adapter.ConnectionRouterEx
	logger    logger.ContextLogger
	listener  *listener.Listener
	service   *anytls.Service
	transport adapter.V2RayServerTransport
}

func NewInbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.AnyTLS2InboundOptions) (adapter.Inbound, error) {
	inbound := &Inbound{
		Adapter: inbound.NewAdapter(C.TypeAnyTLS2, tag),
		router:  uot.NewRouter(router, logger),
		logger:  logger,
	}

	if options.TLS != nil && options.TLS.Enabled {
		tlsConfig, err := tls.NewServerWithOptions(tls.ServerOptions{
			Context:        ctx,
			Logger:         logger,
			Options:        common.PtrValueOrDefault(options.TLS),
			KTLSCompatible: common.PtrValueOrDefault(options.Transport).Type == "",
		})
		if err != nil {
			return nil, err
		}
		inbound.tlsConfig = tlsConfig
	}

	paddingScheme := padding.DefaultPaddingScheme
	if len(options.PaddingScheme) > 0 {
		paddingScheme = []byte(strings.Join(options.PaddingScheme, "\n"))
	}
	service, err := anytls.NewService(anytls.ServiceConfig{
		Users: common.Map(options.Users, func(it option.AnyTLSUser) anytls.User {
			return anytls.User(it)
		}),
		PaddingScheme: paddingScheme,
		Handler:       (*inboundHandler)(inbound),
		Logger:        logger,
	})
	if err != nil {
		return nil, err
	}
	inbound.service = service
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
	var tcpListener net.Listener
	var udpConn net.PacketConn
	if common.Contains(h.transport.Network(), N.NetworkTCP) {
		var err error
		tcpListener, err = h.listener.ListenTCP()
		if err != nil {
			return err
		}
	}
	if common.Contains(h.transport.Network(), N.NetworkUDP) {
		var err error
		udpConn, err = h.listener.ListenUDP()
		if err != nil {
			common.Close(tcpListener)
			return err
		}
	}
	if tcpListener != nil {
		go func() {
			sErr := h.transport.Serve(tcpListener)
			if sErr != nil && !E.IsClosed(sErr) {
				h.logger.Error("transport serve error: ", sErr)
			}
		}()
	}
	if udpConn != nil {
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
	return common.Close(h.listener, h.tlsConfig, h.transport)
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

type inboundHandler Inbound

func (h *inboundHandler) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	var metadata adapter.InboundContext
	metadata.Inbound = h.Tag()
	metadata.InboundType = h.Type()
	metadata.InboundDetour = h.listener.ListenOptions().Detour
	metadata.Source = source
	metadata.Destination = destination.Unwrap()
	if userName, _ := auth.UserFromContext[string](ctx); userName != "" {
		metadata.User = userName
		h.logger.InfoContext(ctx, "[", userName, "] inbound connection to ", metadata.Destination)
	} else {
		h.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
	}
	h.router.RouteConnectionEx(ctx, conn, metadata, onClose)
}

type inboundTransportHandler Inbound

func (h *inboundTransportHandler) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	var metadata adapter.InboundContext
	metadata.Source = source
	metadata.Destination = destination
	metadata.InboundDetour = h.listener.ListenOptions().Detour
	h.logger.InfoContext(ctx, "inbound connection from ", metadata.Source)
	(*Inbound)(h).NewConnectionEx(ctx, conn, metadata, onClose)
}
