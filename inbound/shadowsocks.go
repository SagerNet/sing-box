package inbound

import (
	"context"
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/mux"
	"github.com/sagernet/sing-box/common/uot"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-shadowsocks"
	"github.com/sagernet/sing-shadowsocks/shadowaead"
	"github.com/sagernet/sing-shadowsocks/shadowaead_2022"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/ntp"
)

func NewShadowsocks(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.ShadowsocksInboundOptions) (adapter.Inbound, error) {
	if len(options.Users) > 0 && len(options.Destinations) > 0 {
		return nil, E.New("users and destinations options must not be combined")
	}
	if len(options.Users) > 0 {
		return newShadowsocksMulti(ctx, router, logger, tag, options)
	} else if len(options.Destinations) > 0 {
		return newShadowsocksRelay(ctx, router, logger, tag, options)
	} else {
		return newShadowsocks(ctx, router, logger, tag, options)
	}
}

var (
	_ adapter.Inbound              = (*Shadowsocks)(nil)
	_ adapter.TCPInjectableInbound = (*Shadowsocks)(nil)
)

type Shadowsocks struct {
	myInboundAdapter
	service shadowsocks.Service
}

func newShadowsocks(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.ShadowsocksInboundOptions) (*Shadowsocks, error) {
	inbound := &Shadowsocks{
		myInboundAdapter: myInboundAdapter{
			protocol:      C.TypeShadowsocks,
			network:       options.Network.Build(),
			ctx:           ctx,
			router:        uot.NewRouter(router, logger),
			logger:        logger,
			tag:           tag,
			listenOptions: options.ListenOptions,
		},
	}

	inbound.connHandler = inbound
	inbound.packetHandler = inbound
	var err error
	inbound.router, err = mux.NewRouterWithOptions(inbound.router, logger, common.PtrValueOrDefault(options.Multiplex))
	if err != nil {
		return nil, err
	}

	var udpTimeout time.Duration
	if options.UDPTimeout != 0 {
		udpTimeout = time.Duration(options.UDPTimeout)
	} else {
		udpTimeout = C.UDPTimeout
	}
	switch {
	case options.Method == shadowsocks.MethodNone:
		inbound.service = shadowsocks.NewNoneService(int64(udpTimeout.Seconds()), adapter.NewUpstreamHandler(adapter.InboundContext{}, inbound.newConnection, inbound.newPacketConnection, inbound))
	case common.Contains(shadowaead.List, options.Method):
		inbound.service, err = shadowaead.NewService(options.Method, nil, options.Password, int64(udpTimeout.Seconds()), adapter.NewUpstreamHandler(adapter.InboundContext{}, inbound.newConnection, inbound.newPacketConnection, inbound))
	case common.Contains(shadowaead_2022.List, options.Method):
		inbound.service, err = shadowaead_2022.NewServiceWithPassword(options.Method, options.Password, int64(udpTimeout.Seconds()), adapter.NewUpstreamHandler(adapter.InboundContext{}, inbound.newConnection, inbound.newPacketConnection, inbound), ntp.TimeFuncFromContext(ctx))
	default:
		err = E.New("unsupported method: ", options.Method)
	}
	inbound.packetUpstream = inbound.service
	return inbound, err
}

func (h *Shadowsocks) NewConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	err := h.service.NewConnection(ctx, conn, adapter.UpstreamMetadata(metadata))
	N.CloseOnHandshakeFailure(conn, onClose, err)
	if err != nil {
		h.logger.ErrorContext(ctx, E.Cause(err, "process connection from ", metadata.Source))
	}
}

func (h *Shadowsocks) NewPacketEx(buffer *buf.Buffer, source M.Socksaddr) {
	err := h.service.NewPacket(h.ctx, h.packetConn(), buffer, M.Metadata{Source: source})
	if err != nil {
		h.logger.Error(E.Cause(err, "process packet from ", source))
	}
}

func (h *Shadowsocks) newConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	h.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
	return h.router.RouteConnection(ctx, conn, h.createMetadata(conn, metadata))
}

func (h *Shadowsocks) newPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	ctx = log.ContextWithNewID(ctx)
	h.logger.InfoContext(ctx, "inbound packet connection from ", metadata.Source)
	h.logger.InfoContext(ctx, "inbound packet connection to ", metadata.Destination)
	return h.router.RoutePacketConnection(ctx, conn, h.createPacketMetadata(conn, metadata))
}
