package inbound

import (
	std_bufio "bufio"
	"context"
	"net"
	"os"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/rw"
	"github.com/sagernet/sing/protocol/http"
	"github.com/sagernet/sing/protocol/socks"
	"github.com/sagernet/sing/protocol/socks/socks4"
	"github.com/sagernet/sing/protocol/socks/socks5"
)

var (
	_ adapter.Inbound           = (*Mixed)(nil)
	_ adapter.InjectableInbound = (*Mixed)(nil)
)

type Mixed struct {
	myInboundAdapter
	authenticator auth.Authenticator
}

func NewMixed(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.HTTPMixedInboundOptions) *Mixed {
	inbound := &Mixed{
		myInboundAdapter{
			protocol:       C.TypeMixed,
			network:        []string{N.NetworkTCP},
			ctx:            ctx,
			router:         router,
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

func (h *Mixed) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	headerType, err := rw.ReadByte(conn)
	if err != nil {
		return err
	}
	switch headerType {
	case socks4.Version, socks5.Version:
		return socks.HandleConnection0(ctx, conn, headerType, h.authenticator, h.upstreamUserHandler(metadata), adapter.UpstreamMetadata(metadata))
	}
	reader := std_bufio.NewReader(bufio.NewCachedReader(conn, buf.As([]byte{headerType})))
	return http.HandleConnection(ctx, conn, reader, h.authenticator, h.upstreamUserHandler(metadata), adapter.UpstreamMetadata(metadata))
}

func (h *Mixed) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return os.ErrInvalid
}
