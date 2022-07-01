package inbound

import (
	std_bufio "bufio"
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/config"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common/auth"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/protocol/http"
)

var _ adapter.Inbound = (*HTTP)(nil)

type HTTP struct {
	myInboundAdapter
	authenticator auth.Authenticator
}

func NewHTTP(ctx context.Context, router adapter.Router, logger log.Logger, tag string, options *config.SimpleInboundOptions) *HTTP {
	inbound := &HTTP{
		myInboundAdapter{
			protocol:      C.TypeHTTP,
			network:       []string{C.NetworkTCP},
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

func (h *HTTP) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return http.HandleConnection(ctx, conn, std_bufio.NewReader(conn), h.authenticator, h.upstreamHandler(metadata), M.Metadata{})
}
