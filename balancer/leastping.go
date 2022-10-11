package balancer

import (
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

// NewLeastPing creates a new LeastPing outbound
func NewLeastPing(
	router adapter.Router, logger log.ContextLogger,
	options option.BalancerOutboundOptions,
) (Balancer, error) {
	return newRTTBasedBalancer(
		router, logger, options,
		func(node *Node) time.Duration {
			return node.Average
		},
	)
}
