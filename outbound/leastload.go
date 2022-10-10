package outbound

import (
	"context"
	"math/rand"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/balancer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var (
	_ adapter.Outbound      = (*LeastLoad)(nil)
	_ adapter.OutboundGroup = (*LeastLoad)(nil)
)

// LeastLoad is a outbound group that picks outbound with least load
type LeastLoad struct {
	myOutboundAdapter
	options option.LeastLoadOutboundOptions

	*balancer.LeastLoad
	nodes    []*balancer.Node
	fallback adapter.Outbound
}

// NewLeastLoad creates a new LeastLoad outbound
func NewLeastLoad(router adapter.Router, logger log.ContextLogger, tag string, options option.LeastLoadOutboundOptions) (*LeastLoad, error) {
	outbound := &LeastLoad{
		myOutboundAdapter: myOutboundAdapter{
			protocol: C.TypeLeastLoad,
			router:   router,
			logger:   logger,
			tag:      tag,
		},
		options: options,
		nodes:   make([]*balancer.Node, 0, len(options.Outbounds)),
	}
	if len(options.Outbounds) == 0 {
		return nil, E.New("missing tags")
	}
	return outbound, nil
}

// Network implements adapter.Outbound
func (s *LeastLoad) Network() []string {
	picked := s.pick()
	if picked == nil {
		return []string{N.NetworkTCP, N.NetworkUDP}
	}
	return picked.Network()
}

// Start implements common.Starter
func (s *LeastLoad) Start() error {
	for i, tag := range s.options.Outbounds {
		outbound, loaded := s.router.Outbound(tag)
		if !loaded {
			return E.New("outbound ", i, " not found: ", tag)
		}
		s.nodes = append(s.nodes, balancer.NewNode(outbound))
	}
	if s.options.Fallback != "" {
		outbound, loaded := s.router.Outbound(s.options.Fallback)
		if !loaded {
			return E.New("fallback outbound not found: ", s.options.Fallback)
		}
		s.fallback = outbound
	}
	var err error
	s.LeastLoad, err = balancer.NewLeastLoad(s.nodes, s.logger, s.options)
	if err != nil {
		return err
	}
	s.HealthCheck.Start()
	return nil
}

// Now implements adapter.OutboundGroup
func (s *LeastLoad) Now() string {
	return s.pick().Tag()
}

// All implements adapter.OutboundGroup
func (s *LeastLoad) All() []string {
	return s.options.Outbounds
}

// DialContext implements adapter.Outbound
func (s *LeastLoad) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return s.pick().DialContext(ctx, network, destination)
}

// ListenPacket implements adapter.Outbound
func (s *LeastLoad) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return s.pick().ListenPacket(ctx, destination)
}

// NewConnection implements adapter.Outbound
func (s *LeastLoad) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return s.pick().NewConnection(ctx, conn, metadata)
}

// NewPacketConnection implements adapter.Outbound
func (s *LeastLoad) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return s.pick().NewPacketConnection(ctx, conn, metadata)
}

func (s *LeastLoad) pick() adapter.Outbound {
	selects := s.nodes
	if s.LeastLoad != nil {
		selects = s.LeastLoad.Select()
	}
	count := len(selects)
	if count == 0 {
		// goes to fallbackTag
		return s.fallback
	}
	picked := selects[rand.Intn(count)]
	return picked.Outbound
}
