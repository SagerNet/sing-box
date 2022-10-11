package balancer

import (
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

var _ Balancer = (*rttBasedBalancer)(nil)

// Balancer is interface for load balancers
type Balancer interface {
	Pick() *Node
}

type rttBasedBalancer struct {
	nodes   []*Node
	rttFunc rttFunc
	options *option.BalancerOutboundOptions

	*HealthCheck
	costs *WeightManager
}

type rttFunc func(node *Node) time.Duration

// newRTTBasedLoad creates a new rtt based load balancer
func newRTTBasedBalancer(
	nodes []*Node, logger log.ContextLogger,
	options option.BalancerOutboundOptions,
	rttFunc rttFunc,
) (Balancer, error) {
	return &rttBasedBalancer{
		nodes:       nodes,
		rttFunc:     rttFunc,
		options:     &options,
		HealthCheck: NewHealthCheck(nodes, logger, &options.Check),
		costs: NewWeightManager(
			logger, options.Pick.Costs, 1,
			func(value, cost float64) float64 {
				return value * math.Pow(cost, 0.5)
			},
		),
	}, nil
}

// Select selects qualified nodes
func (s *rttBasedBalancer) Pick() *Node {
	nodes := s.HealthCheck.NodesByCategory()
	var candidates []*Node
	if len(nodes.Qualified) > 0 {
		candidates = nodes.Qualified
		for _, node := range candidates {
			node.Weighted = time.Duration(s.costs.Apply(node.Outbound.Tag(), float64(s.rttFunc(node))))
		}
		sortNodes(candidates)
	} else {
		candidates = nodes.Untested
		shuffleNodes(candidates)
	}
	selects := selectNodes(
		candidates, s.logger,
		int(s.options.Pick.Expected), s.options.Pick.Baselines,
	)
	count := len(selects)
	if count == 0 {
		return nil
	}
	return selects[rand.Intn(count)]
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
