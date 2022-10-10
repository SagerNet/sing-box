package balancer

import (
	"github.com/sagernet/sing-box/adapter"
)

var healthPingStatsZero = HealthCheckStats{
	applied: rttUntested,
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
		HealthCheckStats: healthPingStatsZero,
	}
}

// FetchStats fetches statistics from *HealthPing p
func (s *Node) FetchStats(p *HealthCheck) {
	if p == nil || p.Results == nil {
		s.HealthCheckStats = healthPingStatsZero
		return
	}
	r, ok := p.Results[s.Outbound.Tag()]
	if !ok {
		s.HealthCheckStats = healthPingStatsZero
		return
	}
	s.HealthCheckStats = *r.Get()
}
