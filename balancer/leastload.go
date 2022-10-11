package balancer

import (
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

var _ Balancer = (*LeastLoad)(nil)

// LeastLoad is leastload balancer
type LeastLoad struct {
	nodes   []*Node
	options *option.LeastLoadOutboundOptions

	*HealthCheck
	costs *WeightManager
}

// NewLeastLoad creates a new LeastLoad outbound
func NewLeastLoad(
	nodes []*Node, logger log.ContextLogger,
	options option.LeastLoadOutboundOptions,
) (Balancer, error) {
	return &LeastLoad{
		nodes:       nodes,
		options:     &options,
		HealthCheck: NewHealthCheck(nodes, logger, &options.HealthCheck),
		costs: NewWeightManager(
			logger, options.Costs, 1,
			func(value, cost float64) float64 {
				return value * math.Pow(cost, 0.5)
			},
		),
	}, nil
}

// Select selects qualified nodes
func (s *LeastLoad) Select() *Node {
	nodes := s.HealthCheck.NodesByCategory()
	var candidates []*Node
	if len(nodes.Qualified) > 0 {
		candidates := nodes.Qualified
		appliyCost(candidates, s.costs)
		leastPingSort(candidates)
	} else {
		candidates = nodes.Untested
		shuffle(candidates)
	}
	selects := s.selectLeastLoad(candidates)
	count := len(selects)
	if count == 0 {
		return nil
	}
	return selects[rand.Intn(count)]
}

// selectLeastLoad selects nodes according to Baselines and Expected Count.
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
func (s *LeastLoad) selectLeastLoad(nodes []*Node) []*Node {
	if len(nodes) == 0 {
		// s.logger.Debug("LeastLoad: no qualified nodes")
		return nil
	}
	expected := int(s.options.Expected)
	availableCount := len(nodes)
	if expected > availableCount {
		return nodes
	}

	if expected <= 0 {
		expected = 1
	}
	if len(s.options.Baselines) == 0 {
		return nodes[:expected]
	}

	count := 0
	// go through all base line until find expected selects
	for _, b := range s.options.Baselines {
		baseline := time.Duration(b)
		for i := 0; i < availableCount; i++ {
			if nodes[i].Weighted > baseline {
				break
			}
			count = i + 1
		}
		// don't continue if find expected selects
		if count >= expected {
			s.logger.Debug("applied baseline: ", baseline)
			break
		}
	}
	if s.options.Expected > 0 && count < expected {
		count = expected
	}
	return nodes[:count]
}

func appliyCost(nodes []*Node, costs *WeightManager) {
	for _, node := range nodes {
		node.Weighted = time.Duration(costs.Apply(node.Outbound.Tag(), float64(node.Deviation)))
	}
}

func leastloadSort(nodes []*Node) {
	sort.Slice(nodes, func(i, j int) bool {
		left := nodes[i]
		right := nodes[j]
		if left.Weighted != right.Weighted {
			return left.Weighted < right.Weighted
		}
		if left.Deviation != right.Deviation {
			return left.Deviation < right.Deviation
		}
		if left.Average != right.Average {
			return left.Average < right.Average
		}
		if left.Fail != right.Fail {
			return left.Fail < right.Fail
		}
		return left.All > right.All
	})
}
