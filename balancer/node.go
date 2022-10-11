package balancer

import (
	"github.com/sagernet/sing-box/adapter"
)

var healthPingStatsUntested = HealthCheckStats{
	All:       0,
	Fail:      0,
	Deviation: rttUntested,
	Average:   rttUntested,
	Max:       rttUntested,
	Min:       rttUntested,
}

// Node is a banalcer node with health check result
type Node struct {
	Outbound adapter.Outbound
	HealthCheckStats
}

// NewNode creates a new balancer node from outbound
func NewNode(outbound adapter.Outbound) *Node {
	return &Node{
		Outbound:         outbound,
		HealthCheckStats: healthPingStatsUntested,
	}
}
