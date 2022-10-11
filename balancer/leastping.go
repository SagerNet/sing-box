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
	nodes := s.HealthCheck.NodesByCategory()
	var candidates []*Node
	if len(nodes.Qualified) > 0 {
		candidates := nodes.Qualified
		leastPingSort(candidates)
	} else {
		candidates = nodes.Untested
		shuffle(candidates)
	}
	if len(candidates) == 0 {
		return nil
	}
	return candidates[0]
}

func leastPingSort(nodes []*Node) {
	sort.Slice(nodes, func(i, j int) bool {
		left := nodes[i]
		right := nodes[j]
		if left.Average != right.Average {
			return left.Average < right.Average
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
