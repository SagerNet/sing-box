package inbound

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/auth"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"
)

var _ adapter.Inbound = (*Socks)(nil)

type Socks struct {
	myInboundAdapter
	authenticator auth.Authenticator
}

func NewSocks(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.SocksInboundOptions) *Socks {
	inbound := &Socks{
		myInboundAdapter{
			protocol:      C.TypeSocks,
			network:       []string{N.NetworkTCP},
			ctx:           ctx,
			router:        router,
			logger:        logger,
			tag:           tag,
			listenOptions: options.ListenOptions,
		},
		auth.NewAuthenticator(options.Users),
	}
	inbound.connHandler = inbound
	return inbound
}

func (h *Socks) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return socks.HandleConnection(ctx, conn, h.authenticator, h.upstreamUserHandler(metadata), adapter.UpstreamMetadata(metadata))
}
