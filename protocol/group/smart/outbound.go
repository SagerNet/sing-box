package smart

import (
	"context"
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/interrupt"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/protocol/group/smart/engine"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

// RegisterOutbound registers type "smart" on the outbound registry.
// dart-smart:register
func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register[option.SmartOutboundOptions](registry, C.TypeSmart, NewOutbound)
}

var (
	_ adapter.OutboundGroup             = (*Outbound)(nil)
	_ adapter.ConnectionHandlerEx       = (*Outbound)(nil)
	_ adapter.PacketConnectionHandlerEx = (*Outbound)(nil)
)

// Outbound is a Smart group: ordered dial attempts with connection-level failover.
type Outbound struct {
	outbound.Adapter
	ctx                          context.Context
	outboundManager              adapter.OutboundManager
	connection                   adapter.ConnectionManager
	logger                       log.ContextLogger
	tags                         []string
	outbounds                    map[string]adapter.Outbound
	engine                       *engine.Engine
	selected                     string
	interruptGroup               *interrupt.Group
	interruptExternalConnections bool
}

func NewOutbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.SmartOutboundOptions) (adapter.Outbound, error) {
	if len(options.Outbounds) == 0 {
		return nil, E.New("missing outbounds")
	}
	return &Outbound{
		Adapter:                      outbound.NewAdapter(C.TypeSmart, tag, []string{N.NetworkTCP, N.NetworkUDP}, options.Outbounds),
		ctx:                          ctx,
		outboundManager:              service.FromContext[adapter.OutboundManager](ctx),
		connection:                   service.FromContext[adapter.ConnectionManager](ctx),
		logger:                       logger,
		tags:                         options.Outbounds,
		outbounds:                    make(map[string]adapter.Outbound),
		engine:                       engine.New(options.Outbounds, engine.Options{}),
		interruptGroup:               interrupt.NewGroup(),
		interruptExternalConnections: options.InterruptExistConnections,
	}, nil
}

func (s *Outbound) Start() error {
	for i, tag := range s.tags {
		detour, loaded := s.outboundManager.Outbound(tag)
		if !loaded {
			return E.New("outbound ", i, " not found: ", tag)
		}
		s.outbounds[tag] = detour
	}
	s.selected = s.tags[0]
	return nil
}

func (s *Outbound) Close() error {
	return nil
}

func (s *Outbound) Now() string {
	if s.selected != "" {
		return s.selected
	}
	if len(s.tags) > 0 {
		return s.tags[0]
	}
	return ""
}

func (s *Outbound) All() []string {
	return s.tags
}

func (s *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	host := destination.Fqdn
	if host == "" {
		host = destination.AddrString()
	}
	var lastErr error
	for _, candidate := range s.engine.Select(host) {
		detour := s.outbounds[candidate.Tag]
		if detour == nil {
			continue
		}
		if !common.Contains(detour.Network(), N.NetworkName(network)) {
			continue
		}
		start := time.Now()
		conn, err := detour.DialContext(ctx, network, destination)
		rttMs := float64(time.Since(start).Milliseconds())
		if err != nil {
			s.engine.Record(candidate.Tag, engine.OutcomeFailure, rttMs)
			s.logger.DebugContext(ctx, "smart dial failed via ", candidate.Tag, ": ", err)
			lastErr = err
			continue
		}
		// Soft-fail: dial itself was slow vs history (handshake-style signal).
		threshold := s.engine.SoftFailThresholdMs(candidate.Tag)
		if rttMs > threshold && threshold > 0 {
			_ = conn.Close()
			s.engine.Record(candidate.Tag, engine.OutcomeSoftFail, rttMs)
			s.logger.DebugContext(ctx, "smart soft-fail via ", candidate.Tag, " rtt=", int(rttMs), "ms threshold=", int(threshold), "ms")
			lastErr = E.New("smart soft-fail: ", candidate.Tag)
			continue
		}
		s.engine.Record(candidate.Tag, engine.OutcomeSuccess, rttMs)
		s.engine.RememberHost(host, candidate.Tag)
		s.selected = candidate.Tag
		return s.interruptGroup.NewConn(conn, interrupt.IsExternalConnectionFromContext(ctx)), nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, E.New("smart: no usable outbound")
}

func (s *Outbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	host := destination.Fqdn
	if host == "" {
		host = destination.AddrString()
	}
	var lastErr error
	for _, candidate := range s.engine.Select(host) {
		detour := s.outbounds[candidate.Tag]
		if detour == nil {
			continue
		}
		if !common.Contains(detour.Network(), N.NetworkUDP) {
			continue
		}
		start := time.Now()
		conn, err := detour.ListenPacket(ctx, destination)
		rttMs := float64(time.Since(start).Milliseconds())
		if err != nil {
			s.engine.Record(candidate.Tag, engine.OutcomeFailure, rttMs)
			lastErr = err
			continue
		}
		s.engine.Record(candidate.Tag, engine.OutcomeSuccess, rttMs)
		s.engine.RememberHost(host, candidate.Tag)
		s.selected = candidate.Tag
		return s.interruptGroup.NewPacketConn(conn, interrupt.IsExternalConnectionFromContext(ctx)), nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, E.New("smart: no usable outbound for packet")
}

func (s *Outbound) NewConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	ctx = interrupt.ContextWithIsExternalConnection(ctx)
	s.connection.NewConnection(ctx, s, conn, metadata, onClose)
}

func (s *Outbound) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	ctx = interrupt.ContextWithIsExternalConnection(ctx)
	s.connection.NewPacketConnection(ctx, s, conn, metadata, onClose)
}
