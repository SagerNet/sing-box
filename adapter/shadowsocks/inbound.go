package shadowsocks

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/config"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-shadowsocks"
	"github.com/sagernet/sing-shadowsocks/shadowaead"
	"github.com/sagernet/sing-shadowsocks/shadowaead_2022"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var ErrUnsupportedMethod = E.New("unsupported method")

var _ adapter.InboundHandler = (*Inbound)(nil)

type Inbound struct {
	router  adapter.Router
	logger  log.Logger
	network []string
	service shadowsocks.Service
}

func (i *Inbound) Network() []string {
	return i.network
}

func NewInbound(router adapter.Router, logger log.Logger, options *config.ShadowsocksInboundOptions) (inbound *Inbound, err error) {
	inbound = &Inbound{
		router:  router,
		logger:  logger,
		network: options.Network.Build(),
	}
	handler := (*inboundHandler)(inbound)

	var udpTimeout int64
	if options.UDPTimeout != 0 {
		udpTimeout = options.UDPTimeout
	} else {
		udpTimeout = 300
	}

	switch {
	case options.Method == shadowsocks.MethodNone:
		inbound.service = shadowsocks.NewNoneService(options.UDPTimeout, handler)
	case common.Contains(shadowaead.List, options.Method):
		inbound.service, err = shadowaead.NewService(options.Method, nil, options.Password, udpTimeout, handler)
	case common.Contains(shadowaead_2022.List, options.Method):
		inbound.service, err = shadowaead_2022.NewServiceWithPassword(options.Method, options.Password, udpTimeout, handler)
	default:
		err = E.Extend(ErrUnsupportedMethod, options.Method)
	}
	return
}

func (i *Inbound) Type() string {
	return C.TypeShadowsocks
}

func (i *Inbound) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return i.service.NewConnection(&inboundContext{ctx, metadata}, conn, M.Metadata{
		Source: M.SocksaddrFromNetIP(metadata.Source),
	})
}

func (i *Inbound) NewPacket(ctx context.Context, conn N.PacketConn, buffer *buf.Buffer, metadata adapter.InboundContext) error {
	return i.service.NewPacket(&inboundContext{ctx, metadata}, conn, buffer, M.Metadata{
		Source: M.SocksaddrFromNetIP(metadata.Source),
	})
}

func (i *Inbound) Upstream() any {
	return i.service
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
	ctx = log.ContextWithID(inboundCtx.Context)
	h.logger.WithContext(ctx).Info("inbound packet connection from ", inboundCtx.metadata.Source)
	h.logger.WithContext(ctx).Info("inbound packet connection to ", metadata.Destination)
	inboundCtx.metadata.Destination = metadata.Destination
	return h.router.RoutePacketConnection(ctx, conn, inboundCtx.metadata)
}

func (h *inboundHandler) NewError(ctx context.Context, err error) {
	common.Close(err)
	if E.IsClosed(err) {
		return
	}
	h.logger.WithContext(ctx).Warn(err)
}
