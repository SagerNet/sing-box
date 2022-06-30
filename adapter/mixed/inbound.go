package mixed

import (
	std_bufio "bufio"
	"context"
	"net"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/config"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/rw"
	"github.com/sagernet/sing/protocol/http"
	"github.com/sagernet/sing/protocol/socks"
	"github.com/sagernet/sing/protocol/socks/socks4"
	"github.com/sagernet/sing/protocol/socks/socks5"
)

var _ adapter.InboundHandler = (*Inbound)(nil)

type Inbound struct {
	router        adapter.Router
	logger        log.Logger
	authenticator auth.Authenticator
}

func NewInbound(router adapter.Router, logger log.Logger, options *config.SimpleInboundOptions) *Inbound {
	return &Inbound{
		router:        router,
		logger:        logger,
		authenticator: auth.NewAuthenticator(options.Users),
	}
}

func (i *Inbound) Type() string {
	return C.TypeMixed
}

func (i *Inbound) Network() []string {
	return []string{C.NetworkTCP}
}

func (i *Inbound) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	headerType, err := rw.ReadByte(conn)
	if err != nil {
		return err
	}
	ctx = &inboundContext{ctx, metadata}
	switch headerType {
	case socks4.Version, socks5.Version:
		return socks.HandleConnection0(ctx, conn, headerType, i.authenticator, (*inboundHandler)(i), M.Metadata{})
	}
	reader := std_bufio.NewReader(bufio.NewCachedReader(conn, buf.As([]byte{headerType})))
	return http.HandleConnection(ctx, conn, reader, i.authenticator, (*inboundHandler)(i), M.Metadata{})
}

func (i *Inbound) NewPacket(ctx context.Context, conn N.PacketConn, buffer *buf.Buffer, metadata adapter.InboundContext) error {
	return os.ErrInvalid
}

type inboundContext struct {
	context.Context
	metadata adapter.InboundContext
}

type inboundHandler Inbound

func (h *inboundHandler) NewConnection(ctx context.Context, conn net.Conn, metadata M.Metadata) error {
	inboundCtx, _ := common.Cast[*inboundContext](ctx)
	ctx = inboundCtx.Context
	h.logger.WithContext(ctx).Info("inbound connection to ", metadata.Destination)
	inboundCtx.metadata.Destination = metadata.Destination
	return h.router.RouteConnection(ctx, conn, inboundCtx.metadata)
}

func (h *inboundHandler) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata M.Metadata) error {
	inboundCtx, _ := common.Cast[*inboundContext](ctx)
	ctx = inboundCtx.Context
	h.logger.WithContext(ctx).Info("inbound packet connection to ", metadata.Destination)
	inboundCtx.metadata.Destination = metadata.Destination
	return h.router.RoutePacketConnection(ctx, conn, inboundCtx.metadata)
}
