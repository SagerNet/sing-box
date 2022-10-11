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
	Select() *Node
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
func (s *rttBasedBalancer) Select() *Node {
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
	selects := pickNodes(
		candidates, s.logger,
		int(s.options.Pick.Expected), s.options.Pick.Baselines,
	)
	count := len(selects)
	if count == 0 {
		return nil
	}
	return selects[rand.Intn(count)]
}

// pickNodes selects nodes according to Baselines and Expected Count.
//
// The strategy always improves network response speed, not matter which mode below is configurated.
// But they can still have different priorities.
//
// 1. Bandwidth priority: no Baseline + Expected Count > 0.: selects `Expected Count` of nodes.
// (one if Expected Count <= 0)
//
// 2. Bandwidth priority advanced: Baselines + Expected Count > 0.
// Select `Expected Count` amount of nodes, and also those near them according to baselines.
// In other words, it selects according to different Baselines, until one of them matches
// the Expected Count, if no Baseline matches, Expected Count applied.
//
// 3. Speed priority: Baselines + `Expected Count <= 0`.
// go through all baselines until find selects, if not, select none. Used in combination
// with 'balancer.fallbackTag', it means: selects qualified nodes or use the fallback.
func pickNodes(nodes []*Node, logger log.Logger, expected int, baselines []option.Duration) []*Node {
	if len(nodes) == 0 {
		// s.logger.Debug("no qualified nodes")
		return nil
	}
	expected2 := int(expected)
	availableCount := len(nodes)
	if expected2 > availableCount {
		return nodes
	}

	if expected2 <= 0 {
		expected2 = 1
	}
	if len(baselines) == 0 {
		return nodes[:expected2]
	}

	count := 0
	// go through all base line until find expected selects
	for _, b := range baselines {
		baseline := time.Duration(b)
		for i := 0; i < availableCount; i++ {
			if nodes[i].Weighted > baseline {
				break
			}
			count = i + 1
		}
		// don't continue if find expected selects
		if count >= expected2 {
			logger.Debug("applied baseline: ", baseline)
			break
		}
	}
	if expected > 0 && count < expected2 {
		count = expected2
	}
	return nodes[:count]
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
