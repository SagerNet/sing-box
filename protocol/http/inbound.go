package http

import (
	"context"
	"io"
	"net"
	"slices"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/common/listener"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/common/uot"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/http"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
)

func RegisterInbound(registry *inbound.Registry) {
	inbound.Register[option.HTTPInboundOptions](registry, C.TypeHTTP, NewInbound)
}

var _ adapter.TCPInjectableInbound = (*Inbound)(nil)

type Inbound struct {
	inbound.Adapter
	ctx         context.Context
	router      adapter.ConnectionRouterEx
	logger      log.ContextLogger
	listener    *listener.Listener
	server      *http.Server
	tlsConfig   tls.ServerConfig
	http3       bool
	quicOptions option.QUICOptions
	http3Server io.Closer
}

func NewInbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.HTTPInboundOptions) (adapter.Inbound, error) {
	versions := options.Versions()
	serveHTTP1 := slices.Contains(versions, 1)
	serveHTTP2 := slices.Contains(versions, 2)
	serveHTTP3 := slices.Contains(versions, 3)
	if serveHTTP3 && (options.TLS == nil || !options.TLS.Enabled) {
		return nil, E.New("TLS is required for HTTP/3")
	}
	if !serveHTTP1 && !serveHTTP2 && options.SetSystemProxy {
		return nil, E.New("set_system_proxy requires HTTP/1 or HTTP/2")
	}
	inbound := &Inbound{
		Adapter: inbound.NewAdapter(C.TypeHTTP, tag),
		ctx:     ctx,
		router:  uot.NewRouter(router, logger),
		logger:  logger,
		server: http.NewServer(http.ServerOptions{
			Authenticator: auth.NewAuthenticator(options.Users),
			Logger:        logger,
			HTTP1:         serveHTTP1,
			HTTP2:         serveHTTP2,
			HTTP2Options:  options.HTTP2Options,
			UDP:           true,
		}),
		http3:       serveHTTP3,
		quicOptions: options.HTTP3Options,
	}
	if options.TLS != nil {
		tlsConfig, err := tls.NewServerWithOptions(tls.ServerOptions{
			Context:        ctx,
			Logger:         logger,
			Options:        common.PtrValueOrDefault(options.TLS),
			KTLSCompatible: true,
		})
		if err != nil {
			return nil, err
		}
		if tlsConfig != nil {
			inbound.server.ConfigureTLS(tlsConfig)
		}
		inbound.tlsConfig = tlsConfig
	}
	var network []string
	if serveHTTP1 || serveHTTP2 {
		network = []string{N.NetworkTCP}
	}
	inbound.listener = listener.New(listener.Options{
		Context:           ctx,
		Logger:            logger,
		Network:           network,
		Listen:            options.ListenOptions,
		ConnectionHandler: inbound,
		SetSystemProxy:    options.SetSystemProxy,
		SystemProxySOCKS:  false,
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
			return E.Cause(err, "create TLS config")
		}
	}
	err := h.listener.Start()
	if err != nil {
		return err
	}
	if h.http3 {
		return h.startHTTP3()
	}
	return nil
}

func (h *Inbound) startHTTP3() error {
	var metadata adapter.InboundContext
	//nolint:staticcheck
	metadata.InboundDetour = h.listener.ListenOptions().Detour
	http3Server, err := h.server.ListenHTTP3(h.ctx, h.logger, h.listener, adapter.NewUpstreamHandler(metadata, h.newUserConnection, h.streamUserPacketConnection), h.tlsConfig, h.quicOptions)
	if err != nil {
		return err
	}
	h.http3Server = http3Server
	return nil
}

func (h *Inbound) Close() error {
	return common.Close(
		h.listener,
		h.http3Server,
		h.tlsConfig,
	)
}

func (h *Inbound) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	if h.tlsConfig != nil {
		tlsConn, err := tls.ServerHandshake(ctx, conn, h.tlsConfig)
		if err != nil {
			N.CloseOnHandshakeFailure(conn, onClose, err)
			h.logger.ErrorContext(ctx, E.Cause(err, "process connection from ", metadata.Source, ": TLS handshake"))
			return
		}
		conn = tlsConn
	}
	h.server.ServeConnection(ctx, conn, http.NewReader(conn), adapter.NewUpstreamHandler(metadata, h.newUserConnection, h.streamUserPacketConnection), metadata.Source, onClose)
}

func (h *Inbound) newUserConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	metadata.Inbound = h.Tag()
	metadata.InboundType = h.Type()
	user, loaded := auth.UserFromContext[string](ctx)
	if !loaded {
		h.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
		h.router.RouteConnectionEx(ctx, conn, metadata, onClose)
		return
	}
	metadata.User = user
	h.logger.InfoContext(ctx, "[", user, "] inbound connection to ", metadata.Destination)
	h.router.RouteConnectionEx(ctx, conn, metadata, onClose)
}

func (h *Inbound) streamUserPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	metadata.Inbound = h.Tag()
	metadata.InboundType = h.Type()
	user, loaded := auth.UserFromContext[string](ctx)
	if !loaded {
		h.logger.InfoContext(ctx, "inbound packet connection to ", metadata.Destination)
		h.router.RoutePacketConnectionEx(ctx, conn, metadata, onClose)
		return
	}
	metadata.User = user
	h.logger.InfoContext(ctx, "[", user, "] inbound packet connection to ", metadata.Destination)
	h.router.RoutePacketConnectionEx(ctx, conn, metadata, onClose)
}
