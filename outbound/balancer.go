package outbound

import (
	"context"
	"io"
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
	_ adapter.Service       = (*Balancer)(nil)
)

// Balancer is a outbound group that picks outbound with least load
type Balancer struct {
	myOutboundAdapter

	tags        []string
	fallbackTag string

	balancer.Balancer
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
func (s *Balancer) Network() []string {
	if s.Balancer == nil {
		return []string{N.NetworkTCP, N.NetworkUDP}
	}
	fallbackNetworks := s.fallback.Network()
	fallbackTCP := common.Contains(fallbackNetworks, N.NetworkTCP)
	fallbackUDP := common.Contains(fallbackNetworks, N.NetworkUDP)
	if fallbackTCP && fallbackUDP {
		// fallback supports all network, we don't need to ask s.Balancer,
		// we know it can fallback to s.fallback for all networks even if
		// no outbound is available
		return fallbackNetworks
	}

	// ask s.Balancer for available networks
	networks := s.Balancer.Networks()
	switch {
	case fallbackTCP:
		if !common.Contains(networks, N.NetworkUDP) {
			return fallbackNetworks
		}
		return []string{N.NetworkTCP, N.NetworkUDP}
	case fallbackUDP:
		if !common.Contains(networks, N.NetworkTCP) {
			return fallbackNetworks
		}
		return []string{N.NetworkTCP, N.NetworkUDP}
	default:
		// fallback supports no network, return the networks from s.Balancer
		return networks
	}
}

// Now implements adapter.OutboundGroup
func (s *Balancer) Now() string {
	return s.pick(context.Background(), "").Tag()
}

// All implements adapter.OutboundGroup
func (s *Balancer) All() []string {
	return s.tags
}

// DialContext implements adapter.Outbound
func (s *Balancer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return s.pick(ctx, network).DialContext(ctx, network, destination)
}

// ListenPacket implements adapter.Outbound
func (s *Balancer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return s.pick(ctx, N.NetworkUDP).ListenPacket(ctx, destination)
}

// NewConnection implements adapter.Outbound
func (s *Balancer) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return s.pick(ctx, N.NetworkTCP).NewConnection(ctx, conn, metadata)
}

// NewPacketConnection implements adapter.Outbound
func (s *Balancer) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return s.pick(ctx, N.NetworkUDP).NewPacketConnection(ctx, conn, metadata)
}

// Close implements adapter.Service
func (s *Balancer) Close() error {
	if c, ok := s.Balancer.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// Start implements adapter.Service
func (s *Balancer) Start() error {
	// the fallback is required, in case that all outbounds are not available,
	// we can pick it instead of returning nil to avoid panic.
	if s.fallbackTag == "" {
		return E.New("fallback not set")
	}
	if s.Balancer == nil {
		return E.New("balancer not set")
	}
	outbound, loaded := s.router.Outbound(s.fallbackTag)
	if !loaded {
		return E.New("fallback outbound not found: ", s.fallbackTag)
	}
	s.fallback = outbound
	if starter, isStarter := s.Balancer.(common.Starter); isStarter {
		err := starter.Start()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Balancer) pick(ctx context.Context, network string) adapter.Outbound {
	tag := s.pickTag(ctx, network)
	if tag == "" {
		s.logger.DebugContext(ctx, "(network=", network, ", candidates=0) => fallback [", s.fallbackTag, "]")
		return s.fallback
	}
	outbound, ok := s.router.Outbound(tag)
	if !ok {
		s.logger.DebugContext(ctx, "[", tag, "] not exist, fallback to [", s.fallbackTag, "]")
		return s.fallback
	}
	return outbound
}

func (s *Balancer) pickTag(ctx context.Context, network string) string {
	if s.Balancer == nil {
		// not started yet, pick a random one
		return s.randomTag()
	}
	tag := s.Balancer.Pick(ctx, network)
	if tag == "" {
		return ""
	}
	return tag
}

func (s *Balancer) randomTag() string {
	nodes := balancer.CoveredOutbounds(s.router, s.tags)
	count := len(nodes)
	if count == 0 {
		return ""
	}
	return s.tags[rand.Intn(count)]
}
