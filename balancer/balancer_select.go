package balancer

import (
	"time"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

// selectNodes selects nodes according to Baselines and Expected Count.
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
func selectNodes(nodes []*Node, logger log.Logger, expected int, baselines []option.Duration) []*Node {
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
		for i := count; i < availableCount; i++ {
			if nodes[i].Weighted >= baseline {
				break
			}
			count = i + 1
		}
		// don't continue if find expected selects
		if count >= expected2 {
			if logger != nil {
				logger.Debug("applied baseline: ", baseline)
			}
			break
		}
	}
	if expected > 0 && count < expected2 {
		count = expected2
	}
	return nodes[:count]
}
