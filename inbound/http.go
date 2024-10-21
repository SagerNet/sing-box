package inbound

import (
	std_bufio "bufio"
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
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
)

var (
	_ adapter.Inbound              = (*HTTP)(nil)
	_ adapter.TCPInjectableInbound = (*HTTP)(nil)
)

type HTTP struct {
	myInboundAdapter
	authenticator *auth.Authenticator
	tlsConfig     tls.ServerConfig
}

func NewHTTP(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.HTTPMixedInboundOptions) (*HTTP, error) {
	inbound := &HTTP{
		myInboundAdapter: myInboundAdapter{
			protocol:       C.TypeHTTP,
			network:        []string{N.NetworkTCP},
			ctx:            ctx,
			router:         uot.NewRouter(router, logger),
			logger:         logger,
			tag:            tag,
			listenOptions:  options.ListenOptions,
			setSystemProxy: options.SetSystemProxy,
		},
		authenticator: auth.NewAuthenticator(options.Users),
	}
	if options.TLS != nil {
		tlsConfig, err := tls.NewServer(ctx, logger, common.PtrValueOrDefault(options.TLS))
		if err != nil {
			return nil, err
		}
		inbound.tlsConfig = tlsConfig
	}
	inbound.connHandler = inbound
	return inbound, nil
}

func (h *HTTP) Start() error {
	if h.tlsConfig != nil {
		err := h.tlsConfig.Start()
		if err != nil {
			return E.Cause(err, "create TLS config")
		}
	}
	return h.myInboundAdapter.Start()
}

func (h *HTTP) Close() error {
	return common.Close(
		&h.myInboundAdapter,
		h.tlsConfig,
	)
}

func (h *HTTP) NewConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	err := h.newConnection(ctx, conn, metadata, onClose)
	N.CloseOnHandshakeFailure(conn, onClose, err)
	if err != nil {
		h.logger.ErrorContext(ctx, E.Cause(err, "process connection from ", metadata.Source))
	}
}

func (h *HTTP) newConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) error {
	var err error
	if h.tlsConfig != nil {
		conn, err = tls.ServerHandshake(ctx, conn, h.tlsConfig)
		if err != nil {
			return err
		}
	}
	return http.HandleConnectionEx(ctx, conn, std_bufio.NewReader(conn), h.authenticator, nil, h.upstreamUserHandlerEx(metadata), metadata.Source, onClose)
}

func (a *myInboundAdapter) upstreamUserHandlerEx(metadata adapter.InboundContext) adapter.UpstreamHandlerAdapterEx {
	return adapter.NewUpstreamHandlerEx(metadata, a.newUserConnection, a.streamUserPacketConnection)
}

func (a *myInboundAdapter) newUserConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	user, loaded := auth.UserFromContext[string](ctx)
	if !loaded {
		a.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
		a.router.RouteConnectionEx(ctx, conn, metadata, onClose)
		return
	}
	metadata.User = user
	a.logger.InfoContext(ctx, "[", user, "] inbound connection to ", metadata.Destination)
	a.router.RouteConnectionEx(ctx, conn, metadata, onClose)
}

func (a *myInboundAdapter) streamUserPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	user, loaded := auth.UserFromContext[string](ctx)
	if !loaded {
		a.logger.InfoContext(ctx, "inbound packet connection to ", metadata.Destination)
		a.router.RoutePacketConnectionEx(ctx, conn, metadata, onClose)
		return
	}
	metadata.User = user
	a.logger.InfoContext(ctx, "[", user, "] inbound packet connection to ", metadata.Destination)
	a.router.RoutePacketConnectionEx(ctx, conn, metadata, onClose)
}
