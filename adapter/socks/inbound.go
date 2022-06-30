package socks

import (
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
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"
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
	return C.TypeSocks
}

func (i *Inbound) Network() []string {
	return []string{C.NetworkTCP}
}

func (i *Inbound) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	ctx = &inboundContext{ctx, metadata}
	return socks.HandleConnection(ctx, conn, i.authenticator, (*inboundHandler)(i), M.Metadata{})
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
