package balancer

import (
	"time"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

// NewLeastPing creates a new LeastPing outbound
func NewLeastPing(
	nodes []*Node, logger log.ContextLogger,
	options option.BalancerOutboundOptions,
) (Balancer, error) {
	return newRTTBasedBalancer(
		nodes, logger, options,
		func(node *Node) time.Duration {
			return node.Average
		},
	)
}
