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
	"github.com/sagernet/sing/protocol/socks"
)

var (
	_ adapter.Inbound              = (*Socks)(nil)
	_ adapter.TCPInjectableInbound = (*Socks)(nil)
)

type Socks struct {
	myInboundAdapter
	authenticator *auth.Authenticator
}

func NewSocks(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.SocksInboundOptions) *Socks {
	inbound := &Socks{
		myInboundAdapter{
			protocol:      C.TypeSOCKS,
			network:       []string{N.NetworkTCP},
			ctx:           ctx,
			router:        uot.NewRouter(router, logger),
			logger:        logger,
			tag:           tag,
			listenOptions: options.ListenOptions,
		},
		auth.NewAuthenticator(options.Users),
	}
	inbound.connHandler = inbound
	return inbound
}

func (h *Socks) NewConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	err := socks.HandleConnectionEx(ctx, conn, std_bufio.NewReader(conn), h.authenticator, nil, h.upstreamUserHandlerEx(metadata), metadata.Source, metadata.Destination, onClose)
	N.CloseOnHandshakeFailure(conn, onClose, err)
	if err != nil {
		h.logger.ErrorContext(ctx, E.Cause(err, "process connection from ", metadata.Source))
	}
}
