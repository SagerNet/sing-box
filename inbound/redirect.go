package inbound

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/redir"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type Redirect struct {
	myInboundAdapter
}

func NewRedirect(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.RedirectInboundOptions) *Redirect {
	redirect := &Redirect{
		myInboundAdapter{
			protocol:      C.TypeRedirect,
			network:       []string{N.NetworkTCP},
			ctx:           ctx,
			router:        router,
			logger:        logger,
			tag:           tag,
			listenOptions: options.ListenOptions,
		},
	}
	redirect.connHandler = redirect
	return redirect
}

func (r *Redirect) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	destination, err := redir.GetOriginalDestination(conn)
	if err != nil {
		return E.Cause(err, "get redirect destination")
	}
	metadata.Destination = M.SocksaddrFromNetIP(destination)
	return r.newConnection(ctx, conn, metadata)
}
