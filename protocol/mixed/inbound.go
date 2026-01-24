package mixed

import (
	std_bufio "bufio"
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/common/listener"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/common/uot"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/http"
	"github.com/sagernet/sing/protocol/socks"
	"github.com/sagernet/sing/protocol/socks/socks4"
	"github.com/sagernet/sing/protocol/socks/socks5"
)

func RegisterInbound(registry *inbound.Registry) {
	inbound.Register[option.HTTPMixedInboundOptions](registry, C.TypeMixed, NewInbound)
}

var _ adapter.TCPInjectableInbound = (*Inbound)(nil)

type Inbound struct {
	inbound.Adapter
	router        adapter.ConnectionRouterEx
	logger        log.ContextLogger
	listener      *listener.Listener
	authenticator *auth.Authenticator
	tlsConfig     tls.ServerConfig
}

func NewInbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.HTTPMixedInboundOptions) (adapter.Inbound, error) {
	inbound := &Inbound{
		Adapter:       inbound.NewAdapter(C.TypeMixed, tag),
		router:        uot.NewRouter(router, logger),
		logger:        logger,
		authenticator: auth.NewAuthenticator(options.Users),
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
		inbound.tlsConfig = tlsConfig
	}
	inbound.listener = listener.New(listener.Options{
		Context:           ctx,
		Logger:            logger,
		Network:           []string{N.NetworkTCP},
		Listen:            options.ListenOptions,
		ConnectionHandler: inbound,
		SetSystemProxy:    options.SetSystemProxy,
		SystemProxySOCKS:  true,
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
	return h.listener.Start()
}

func (h *Inbound) Close() error {
	return common.Close(
		h.listener,
		h.tlsConfig,
	)
}

func (h *Inbound) NewConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	err := h.newConnection(ctx, conn, metadata, onClose)
	N.CloseOnHandshakeFailure(conn, onClose, err)
	if err != nil {
		if E.IsClosedOrCanceled(err) {
			h.logger.DebugContext(ctx, "connection closed: ", err)
		} else {
			h.logger.ErrorContext(ctx, E.Cause(err, "process connection from ", metadata.Source))
		}
	}
}

func (h *Inbound) newConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) error {
	if h.tlsConfig != nil {
		tlsConn, err := tls.ServerHandshake(ctx, conn, h.tlsConfig)
		if err != nil {
			return E.Cause(err, "TLS handshake")
		}
		conn = tlsConn
	}
	reader := std_bufio.NewReader(conn)
	headerBytes, err := reader.Peek(1)
	if err != nil {
		return E.Cause(err, "peek first byte")
	}
	switch headerBytes[0] {
	case socks4.Version, socks5.Version:
		return socks.HandleConnectionEx(ctx, conn, reader, h.authenticator, adapter.NewUpstreamHandlerEx(metadata, h.newUserConnection, h.streamUserPacketConnection), h.listener, metadata.Source, onClose)
	default:
		return http.HandleConnectionEx(ctx, conn, reader, h.authenticator, adapter.NewUpstreamHandlerEx(metadata, h.newUserConnection, h.streamUserPacketConnection), metadata.Source, onClose)
	}
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
		if !metadata.Destination.IsValid() {
			h.logger.InfoContext(ctx, "inbound packet connection")
		} else {
			h.logger.InfoContext(ctx, "inbound packet connection to ", metadata.Destination)
		}
		h.router.RoutePacketConnectionEx(ctx, conn, metadata, onClose)
		return
	}
	metadata.User = user
	if !metadata.Destination.IsValid() {
		h.logger.InfoContext(ctx, "[", user, "] inbound packet connection")
	} else {
		h.logger.InfoContext(ctx, "[", user, "] inbound packet connection to ", metadata.Destination)
	}
	h.router.RoutePacketConnectionEx(ctx, conn, metadata, onClose)
}
