package outbound

import (
	"context"
	"math/rand"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/balancer"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var (
	_ adapter.Outbound      = (*Balancer)(nil)
	_ adapter.OutboundGroup = (*Balancer)(nil)
)

// Balancer is a outbound group that picks outbound with least load
type Balancer struct {
	myOutboundAdapter

	tags        []string
	fallbackTag string

	balancer.Balancer
	nodes    []*balancer.Node
	fallback adapter.Outbound
}

// NewBalancer creates a new Balancer outbound
func NewBalancer(
	protocol string, router adapter.Router, logger log.ContextLogger, tag string,
	outbounds []string, fallbackTag string,
) *Balancer {
	b := &Balancer{
		myOutboundAdapter: myOutboundAdapter{
			protocol: protocol,
			router:   router,
			logger:   logger,
			tag:      tag,
		},
		tags:        outbounds,
		fallbackTag: fallbackTag,
	}
	return b
}

// Network implements adapter.Outbound
//
// FIXME: logic issue:
// picked node is very likely to be different between first "Network() assetion"
// then "NewConnection()", maybe we need to keep the picked node in the context?
// that requests to change the Network() signature of the interface of
// adapter.Outbound
func (s *Balancer) Network() []string {
	picked := s.pick()
	if picked == nil {
		return []string{N.NetworkTCP, N.NetworkUDP}
	}
	return picked.Network()
}

// Now implements adapter.OutboundGroup
func (s *Balancer) Now() string {
	return s.pick().Tag()
}

// All implements adapter.OutboundGroup
func (s *Balancer) All() []string {
	return s.tags
}

// DialContext implements adapter.Outbound
func (s *Balancer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return s.pick().DialContext(ctx, network, destination)
}

// ListenPacket implements adapter.Outbound
func (s *Balancer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return s.pick().ListenPacket(ctx, destination)
}

// NewConnection implements adapter.Outbound
func (s *Balancer) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return s.pick().NewConnection(ctx, conn, metadata)
}

// NewPacketConnection implements adapter.Outbound
func (s *Balancer) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return s.pick().NewPacketConnection(ctx, conn, metadata)
}

// initialize inits the balancer
func (s *Balancer) initialize() error {
	// the fallback is required, in case that all outbounds are not available,
	// we can pick it instead of returning nil to avoid panic.
	if s.fallbackTag == "" {
		return E.New("fallback not set")
	}
	outbound, loaded := s.router.Outbound(s.fallbackTag)
	if !loaded {
		return E.New("fallback outbound not found: ", s.fallbackTag)
	}
	s.fallback = outbound
	for i, tag := range s.tags {
		outbound, loaded := s.router.Outbound(tag)
		if !loaded {
			return E.New("outbound ", i, " not found: ", tag)
		}
		s.nodes = append(s.nodes, balancer.NewNode(outbound))
	}
	return nil
}

func (s *Balancer) setBalancer(b balancer.Balancer) error {
	s.Balancer = b
	if starter, isStarter := b.(common.Starter); isStarter {
		err := starter.Start()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Balancer) pick() adapter.Outbound {
	if s.Balancer != nil {
		selected := s.Balancer.Select()
		if selected == nil {
			return s.fallback
		}
		return selected.Outbound
	}
	// not started
	count := len(s.nodes)
	if count == 0 {
		// goes to fallback
		return s.fallback
	}
	picked := s.nodes[rand.Intn(count)]
	return picked.Outbound
}
