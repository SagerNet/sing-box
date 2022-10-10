package balancer

import (
	"math"
	"sort"
	"time"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

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
) (*LeastLoad, error) {
	return &LeastLoad{
		nodes:       nodes,
		options:     &options,
		HealthCheck: NewHealthCheck(nodes, logger, options.HealthCheck),
		costs: NewWeightManager(
			logger, options.Costs, 1,
			func(value, cost float64) float64 {
				return value * math.Pow(cost, 0.5)
			},
		),
	}, nil
}

// Select selects qualified nodes
func (s *LeastLoad) Select() []*Node {
	qualified, _ := s.getNodes()
	return s.selectLeastLoad(qualified)
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
			if nodes[i].applied > baseline {
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

func (s *LeastLoad) getNodes() ([]*Node, []*Node) {
	s.HealthCheck.Lock()
	defer s.HealthCheck.Unlock()

	qualified := make([]*Node, 0)
	unqualified := make([]*Node, 0)
	failed := make([]*Node, 0)
	untested := make([]*Node, 0)
	others := make([]*Node, 0)
	for _, node := range s.nodes {
		node.FetchStats(s.HealthCheck)
		switch {
		case node.All == 0:
			node.applied = rttUntested
			untested = append(untested, node)
		case s.options.MaxRTT > 0 && node.Average > time.Duration(s.options.MaxRTT):
			node.applied = rttUnqualified
			unqualified = append(unqualified, node)
		case float64(node.Fail)/float64(node.All) > float64(s.options.Tolerance):
			node.applied = rttFailed
			if node.All-node.Fail == 0 {
				// no good, put them after has-good nodes
				node.applied = rttFailed
				node.Deviation = rttFailed
				node.Average = rttFailed
			}
			failed = append(failed, node)
		default:
			node.applied = time.Duration(s.costs.Apply(node.Outbound.Tag(), float64(node.Deviation)))
			qualified = append(qualified, node)
		}
	}
	if len(qualified) > 0 {
		leastloadSort(qualified)
		others = append(others, unqualified...)
		others = append(others, untested...)
		others = append(others, failed...)
	} else {
		qualified = untested
		others = append(others, unqualified...)
		others = append(others, failed...)
	}
	return qualified, others
}

func leastloadSort(nodes []*Node) {
	sort.Slice(nodes, func(i, j int) bool {
		left := nodes[i]
		right := nodes[j]
		if left.applied != right.applied {
			return left.applied < right.applied
		}
		if left.applied != right.applied {
			return left.applied < right.applied
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
