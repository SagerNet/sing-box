package shadowsocks

import (
	"context"
	"net"
	"os"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/common/listener"
	"github.com/sagernet/sing-box/common/mux"
	"github.com/sagernet/sing-box/common/uot"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-shadowsocks/shadowaead_2022"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.TCPInjectableInbound = (*RelayInbound)(nil)

type RelayInbound struct {
	inbound.Adapter
	ctx          context.Context
	router       adapter.ConnectionRouterEx
	logger       logger.ContextLogger
	listener     *listener.Listener
	service      *shadowaead_2022.RelayService[int]
	destinations []option.ShadowsocksDestination
}

func newRelayInbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.ShadowsocksInboundOptions) (*RelayInbound, error) {
	inbound := &RelayInbound{
		Adapter:      inbound.NewAdapter(C.TypeShadowsocks, tag),
		ctx:          ctx,
		router:       uot.NewRouter(router, logger),
		logger:       logger,
		destinations: options.Destinations,
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
	service, err := shadowaead_2022.NewRelayServiceWithPassword[int](
		options.Method,
		options.Password,
		int64(udpTimeout.Seconds()),
		adapter.NewUpstreamHandler(adapter.InboundContext{}, inbound.newConnection, inbound.newPacketConnection, inbound),
	)
	if err != nil {
		return nil, err
	}
	err = service.UpdateUsersWithPasswords(common.MapIndexed(options.Destinations, func(index int, user option.ShadowsocksDestination) int {
		return index
	}), common.Map(options.Destinations, func(user option.ShadowsocksDestination) string {
		return user.Password
	}), common.Map(options.Destinations, option.ShadowsocksDestination.Build))
	if err != nil {
		return nil, err
	}
	inbound.service = service
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

func (h *RelayInbound) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	return h.listener.Start()
}

func (h *RelayInbound) Close() error {
	return h.listener.Close()
}

//nolint:staticcheck
func (h *RelayInbound) NewConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
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
func (h *RelayInbound) NewPacketEx(buffer *buf.Buffer, source M.Socksaddr) {
	err := h.service.NewPacket(h.ctx, &stubPacketConn{h.listener.PacketWriter()}, buffer, M.Metadata{Source: source})
	if err != nil {
		h.logger.Error(E.Cause(err, "process packet from ", source))
	}
}

func (h *RelayInbound) newConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	destinationIndex, loaded := auth.UserFromContext[int](ctx)
	if !loaded {
		return os.ErrInvalid
	}
	destination := h.destinations[destinationIndex].Name
	if destination == "" {
		destination = F.ToString(destinationIndex)
	} else {
		metadata.User = destination
	}
	h.logger.InfoContext(ctx, "[", destination, "] inbound connection to ", metadata.Destination)
	metadata.Inbound = h.Tag()
	metadata.InboundType = h.Type()
	//nolint:staticcheck
	metadata.InboundDetour = h.listener.ListenOptions().Detour
	//nolint:staticcheck
	metadata.InboundOptions = h.listener.ListenOptions().InboundOptions
	return h.router.RouteConnection(ctx, conn, metadata)
}

func (h *RelayInbound) newPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	destinationIndex, loaded := auth.UserFromContext[int](ctx)
	if !loaded {
		return os.ErrInvalid
	}
	destination := h.destinations[destinationIndex].Name
	if destination == "" {
		destination = F.ToString(destinationIndex)
	} else {
		metadata.User = destination
	}
	ctx = log.ContextWithNewID(ctx)
	h.logger.InfoContext(ctx, "[", destination, "] inbound packet connection from ", metadata.Source)
	h.logger.InfoContext(ctx, "[", destination, "] inbound packet connection to ", metadata.Destination)
	metadata.Inbound = h.Tag()
	metadata.InboundType = h.Type()
	//nolint:staticcheck
	metadata.InboundDetour = h.listener.ListenOptions().Detour
	//nolint:staticcheck
	metadata.InboundOptions = h.listener.ListenOptions().InboundOptions
	return h.router.RoutePacketConnection(ctx, conn, metadata)
}

//nolint:staticcheck
func (h *RelayInbound) NewError(ctx context.Context, err error) {
	NewError(h.logger, ctx, err)
}
