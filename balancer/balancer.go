package balancer

import (
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	N "github.com/sagernet/sing/common/network"
)

var _ Balancer = (*rttBasedBalancer)(nil)

// Balancer is interface for load balancers
type Balancer interface {
	// Pick picks a qualified nodes
	Pick(network string) string
	// Networks returns the supported network types
	Networks() []string
}

type rttBasedBalancer struct {
	nodes   []*Node
	rttFunc rttFunc
	options *option.BalancerOutboundOptions

	*HealthCheck
	costs *WeightManager
}

type rttFunc func(n *Node) time.Duration

// newRTTBasedLoad creates a new rtt based load balancer
func newRTTBasedBalancer(
	router adapter.Router, logger log.ContextLogger,
	options option.BalancerOutboundOptions,
	rttFunc rttFunc,
) (Balancer, error) {
	return &rttBasedBalancer{
		rttFunc:     rttFunc,
		options:     &options,
		HealthCheck: NewHealthCheck(router, options.Outbounds, logger, &options.Check),
		costs: NewWeightManager(
			logger, options.Pick.Costs, 1,
			func(value, cost float64) float64 {
				return value * math.Pow(cost, 0.5)
			},
		),
	}, nil
}

// Select selects qualified nodes
func (s *rttBasedBalancer) Networks() []string {
	hasTCP, hasUDP := false, false
	nodes := s.HealthCheck.Nodes("")
	for _, n := range nodes.Qualified {
		if !hasTCP && common.Contains(n.Networks, N.NetworkTCP) {
			hasTCP = true
		}
		if !hasUDP && common.Contains(n.Networks, N.NetworkUDP) {
			hasUDP = true
		}
		if hasTCP && hasUDP {
			break
		}
	}
	if !hasTCP && !hasUDP {
		for _, n := range nodes.Untested {
			if !hasTCP && common.Contains(n.Networks, N.NetworkTCP) {
				hasTCP = true
			}
			if !hasUDP && common.Contains(n.Networks, N.NetworkUDP) {
				hasUDP = true
			}
			if hasTCP && hasUDP {
				break
			}
		}
	}
	switch {
	case hasTCP && hasUDP:
		return []string{N.NetworkTCP, N.NetworkUDP}
	case hasTCP:
		return []string{N.NetworkTCP}
	case hasUDP:
		return []string{N.NetworkUDP}
	default:
		return nil
	}
}

// Select selects qualified nodes
func (s *rttBasedBalancer) Pick(network string) string {
	nodes := s.HealthCheck.Nodes(network)
	var candidates []*Node
	if len(nodes.Qualified) > 0 {
		candidates = nodes.Qualified
		for _, n := range candidates {
			n.Weighted = time.Duration(s.costs.Apply(n.Tag, float64(s.rttFunc(n))))
		}
		sortNodes(candidates)
	} else {
		candidates = nodes.Untested
		shuffleNodes(candidates)
	}
	selects := selectNodes(candidates, int(s.options.Pick.Expected), s.options.Pick.Baselines)
	count := len(selects)
	if count == 0 {
		return ""
	}
	picked := selects[rand.Intn(count)]
	s.logger.Debug(
		"pick [", picked.Tag, "]",
		" +W=", picked.Weighted,
		" STD=", picked.Deviation,
		" AVG=", picked.Average,
		" Fail=", picked.Fail, "/", picked.All,
	)
	return picked.Tag
}

func sortNodes(nodes []*Node) {
	sort.Slice(nodes, func(i, j int) bool {
		left := nodes[i]
		right := nodes[j]
		if left.Weighted != right.Weighted {
			return left.Weighted < right.Weighted
		}
		if left.Fail != right.Fail {
			return left.Fail < right.Fail
		}
		return left.All > right.All
	})
}

func shuffleNodes(nodes []*Node) {
	rand.Seed(time.Now().Unix())
	rand.Shuffle(len(nodes), func(i, j int) {
		nodes[i], nodes[j] = nodes[j], nodes[i]
	})
}
