package shadowsocks

import (
	"context"
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/common/listener"
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
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/ntp"
)

func RegisterInbound(registry *inbound.Registry) {
	inbound.Register[option.ShadowsocksInboundOptions](registry, C.TypeShadowsocks, NewInbound)
}

func NewInbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.ShadowsocksInboundOptions) (adapter.Inbound, error) {
	if len(options.Users) > 0 && len(options.Destinations) > 0 {
		return nil, E.New("users and destinations options must not be combined")
	} else if options.Managed && (len(options.Users) > 0 || len(options.Destinations) > 0) {
		return nil, E.New("users and destinations options are not supported in managed servers")
	}
	if len(options.Users) > 0 || options.Managed {
		return newMultiInbound(ctx, router, logger, tag, options)
	} else if len(options.Destinations) > 0 {
		return newRelayInbound(ctx, router, logger, tag, options)
	} else {
		return newInbound(ctx, router, logger, tag, options)
	}
}

var _ adapter.TCPInjectableInbound = (*Inbound)(nil)

type Inbound struct {
	inbound.Adapter
	ctx      context.Context
	router   adapter.ConnectionRouterEx
	logger   logger.ContextLogger
	listener *listener.Listener
	service  shadowsocks.Service
}

func newInbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.ShadowsocksInboundOptions) (*Inbound, error) {
	inbound := &Inbound{
		Adapter: inbound.NewAdapter(C.TypeShadowsocks, tag),
		ctx:     ctx,
		router:  uot.NewRouter(router, logger),
		logger:  logger,
	}
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
	inbound.listener = listener.New(listener.Options{
		Context:                  ctx,
		Logger:                   logger,
		Network:                  options.Network.Build(),
		Listen:                   options.ListenOptions,
		ConnectionHandler:        inbound,
		PacketHandler:            inbound,
		ThreadUnsafePacketWriter: true,
	})
	return inbound, err
}

func (h *Inbound) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	return h.listener.Start()
}

func (h *Inbound) Close() error {
	return h.listener.Close()
}

//nolint:staticcheck
func (h *Inbound) NewConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	err := h.service.NewConnection(ctx, conn, adapter.UpstreamMetadata(metadata))
	N.CloseOnHandshakeFailure(conn, onClose, err)
	if err != nil {
		if E.IsClosedOrCanceled(err) {
			h.logger.DebugContext(ctx, "connection closed: ", err)
		} else {
			h.logger.ErrorContext(ctx, E.Cause(err, "process connection from ", metadata.Source))
		}
	}
}

//nolint:staticcheck
func (h *Inbound) NewPacketEx(buffer *buf.Buffer, source M.Socksaddr) {
	err := h.service.NewPacket(h.ctx, &stubPacketConn{h.listener.PacketWriter()}, buffer, M.Metadata{Source: source})
	if err != nil {
		h.logger.Error(E.Cause(err, "process packet from ", source))
	}
}

func (h *Inbound) newConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	h.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
	metadata.Inbound = h.Tag()
	metadata.InboundType = h.Type()
	return h.router.RouteConnection(ctx, conn, metadata)
}

func (h *Inbound) newPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	ctx = log.ContextWithNewID(ctx)
	h.logger.InfoContext(ctx, "inbound packet connection from ", metadata.Source)
	h.logger.InfoContext(ctx, "inbound packet connection to ", metadata.Destination)
	metadata.Inbound = h.Tag()
	metadata.InboundType = h.Type()
	return h.router.RoutePacketConnection(ctx, conn, metadata)
}

var _ N.PacketConn = (*stubPacketConn)(nil)

type stubPacketConn struct {
	N.PacketWriter
}

func (c *stubPacketConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	panic("stub!")
}

func (c *stubPacketConn) Close() error {
	return nil
}

func (c *stubPacketConn) LocalAddr() net.Addr {
	panic("stub!")
}

func (c *stubPacketConn) SetDeadline(t time.Time) error {
	panic("stub!")
}

func (c *stubPacketConn) SetReadDeadline(t time.Time) error {
	panic("stub!")
}

func (c *stubPacketConn) SetWriteDeadline(t time.Time) error {
	panic("stub!")
}

func (h *Inbound) NewError(ctx context.Context, err error) {
	NewError(h.logger, ctx, err)
}

// Deprecated: remove
func NewError(logger logger.ContextLogger, ctx context.Context, err error) {
	common.Close(err)
	if E.IsClosedOrCanceled(err) {
		logger.DebugContext(ctx, "connection closed: ", err)
		return
	}
	logger.ErrorContext(ctx, err)
}
