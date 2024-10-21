package inbound

import (
	std_bufio "bufio"
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/uot"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/http"
	"github.com/sagernet/sing/protocol/socks"
	"github.com/sagernet/sing/protocol/socks/socks4"
	"github.com/sagernet/sing/protocol/socks/socks5"
)

var (
	_ adapter.Inbound              = (*Mixed)(nil)
	_ adapter.TCPInjectableInbound = (*Mixed)(nil)
)

type Mixed struct {
	myInboundAdapter
	authenticator *auth.Authenticator
}

func NewMixed(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.HTTPMixedInboundOptions) *Mixed {
	inbound := &Mixed{
		myInboundAdapter{
			protocol:       C.TypeMixed,
			network:        []string{N.NetworkTCP},
			ctx:            ctx,
			router:         uot.NewRouter(router, logger),
			logger:         logger,
			tag:            tag,
			listenOptions:  options.ListenOptions,
			setSystemProxy: options.SetSystemProxy,
		},
		auth.NewAuthenticator(options.Users),
	}
	inbound.connHandler = inbound
	return inbound
}

func (h *Mixed) NewConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	err := h.newConnection(ctx, conn, metadata, onClose)
	N.CloseOnHandshakeFailure(conn, onClose, err)
	if err != nil {
		h.logger.ErrorContext(ctx, E.Cause(err, "process connection from ", metadata.Source))
	}
}

func (h *Mixed) newConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) error {
	reader := std_bufio.NewReader(conn)
	headerBytes, err := reader.Peek(1)
	if err != nil {
		return E.Cause(err, "peek first byte")
	}
	switch headerBytes[0] {
	case socks4.Version, socks5.Version:
		return socks.HandleConnectionEx(ctx, conn, reader, h.authenticator, nil, h.upstreamUserHandlerEx(metadata), metadata.Source, metadata.Destination, onClose)
	default:
		return http.HandleConnectionEx(ctx, conn, reader, h.authenticator, nil, h.upstreamUserHandlerEx(metadata), metadata.Source, onClose)
	}
}
