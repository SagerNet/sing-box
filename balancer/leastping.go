package balancer

import (
	"math/rand"
	"sort"
	"time"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

var _ Balancer = (*LeastPing)(nil)

// LeastPing is least ping balancer
type LeastPing struct {
	nodes   []*Node
	options *option.LeastPingOutboundOptions

	*HealthCheck
}

// NewLeastPing creates a new LeastPing outbound
func NewLeastPing(
	nodes []*Node, logger log.ContextLogger,
	options option.LeastPingOutboundOptions,
) (Balancer, error) {
	return &LeastPing{
		nodes:       nodes,
		options:     &options,
		HealthCheck: NewHealthCheck(nodes, logger, &options.HealthCheck),
	}, nil
}

// Select selects least ping node
func (s *LeastPing) Select() *Node {
	qualified, _ := s.getNodes()
	if len(qualified) == 0 {
		return nil
	}
	return qualified[0]
}

func (s *LeastPing) getNodes() ([]*Node, []*Node) {
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
			node.applied = node.Average
			qualified = append(qualified, node)
		}
	}
	if len(qualified) > 0 {
		leastPingSort(qualified)
		others = append(others, unqualified...)
		others = append(others, untested...)
		others = append(others, failed...)
	} else {
		// random node if not tested
		shuffle(untested)
		qualified = untested
		others = append(others, unqualified...)
		others = append(others, failed...)
	}
	return qualified, others
}

func leastPingSort(nodes []*Node) {
	sort.Slice(nodes, func(i, j int) bool {
		left := nodes[i]
		right := nodes[j]
		if left.applied != right.applied {
			return left.applied < right.applied
		}
		if left.Fail != right.Fail {
			return left.Fail < right.Fail
		}
		return left.All > right.All
	})
}

func shuffle(nodes []*Node) {
	rand.Seed(time.Now().Unix())
	rand.Shuffle(len(nodes), func(i, j int) {
		nodes[i], nodes[j] = nodes[j], nodes[i]
	})
}
