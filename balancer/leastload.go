package balancer

import (
	"time"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

// NewLeastLoad creates a new LeastLoad outbound
func NewLeastLoad(
	nodes []*Node, logger log.ContextLogger,
	options option.BalancerOutboundOptions,
) (Balancer, error) {
	return newRTTBasedBalancer(
		nodes, logger, options,
		func(node *Node) time.Duration {
			return node.Deviation
		},
	)
}
