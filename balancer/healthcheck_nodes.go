package balancer

import "time"

// CategorizedNodes holds the categorized nodes
type CategorizedNodes struct {
	Qualified, Unqualified []*Node
	Failed, Untested       []*Node
}

// NodesByCategory returns the categorized nodes
func (h *HealthCheck) NodesByCategory() *CategorizedNodes {
	h.Lock()
	defer h.Unlock()
	if h == nil || h.Results == nil {
		return &CategorizedNodes{
			Untested: h.nodes,
		}
	}
	nodes := &CategorizedNodes{
		Qualified:   make([]*Node, 0, len(h.nodes)),
		Unqualified: make([]*Node, 0, len(h.nodes)),
		Failed:      make([]*Node, 0, len(h.nodes)),
		Untested:    make([]*Node, 0, len(h.nodes)),
	}
	for _, node := range h.nodes {
		r, ok := h.Results[node.Outbound.Tag()]
		if !ok {
			node.HealthCheckStats = healthPingStatsUntested
			continue
		}
		node.HealthCheckStats = r.Get()
		switch {
		case node.HealthCheckStats.All == 0:
			nodes.Untested = append(nodes.Untested, node)
		case node.HealthCheckStats.All == node.HealthCheckStats.Fail,
			float64(node.Fail)/float64(node.All) > float64(h.options.Tolerance):
			nodes.Failed = append(nodes.Failed, node)
		case h.options.MaxRTT > 0 && node.Average > time.Duration(h.options.MaxRTT):
			nodes.Unqualified = append(nodes.Unqualified, node)
		default:
			nodes.Qualified = append(nodes.Qualified, node)
		}
	}
	return nodes
}
