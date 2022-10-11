package balancer

import (
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
)

// CategorizedNodes holds the categorized nodes
type CategorizedNodes struct {
	Qualified, Unqualified []*Node
	Failed, Untested       []*Node
}

// NodesByCategory returns the categorized nodes
func (h *HealthCheck) NodesByCategory() *CategorizedNodes {
	h.Lock()
	defer h.Unlock()
	if h == nil || len(h.results) == 0 {
		return &CategorizedNodes{}
	}
	nodes := &CategorizedNodes{
		Qualified:   make([]*Node, 0, len(h.results)),
		Unqualified: make([]*Node, 0, len(h.results)),
		Failed:      make([]*Node, 0, len(h.results)),
		Untested:    make([]*Node, 0, len(h.results)),
	}
	for tag, result := range h.results {
		node := &Node{
			Tag:      tag,
			RTTStats: result.Get(),
		}
		switch {
		case node.RTTStats.All == 0:
			nodes.Untested = append(nodes.Untested, node)
		case node.RTTStats.All == node.RTTStats.Fail,
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

// CoveredOutbounds returns the outbounds that should covered by health check
func CoveredOutbounds(router adapter.Router, tags []string) []adapter.Outbound {
	outbounds := router.Outbounds()
	nodes := make([]adapter.Outbound, 0, len(outbounds))
	for _, outbound := range outbounds {
		for _, prefix := range tags {
			tag := outbound.Tag()
			if strings.HasPrefix(tag, prefix) {
				nodes = append(nodes, outbound)
			}
		}
	}
	return nodes
}

// refreshNodes matches nodes from router by tag prefix, and refreshes the health check results
func (h *HealthCheck) refreshNodes() []adapter.Outbound {
	h.Lock()
	defer h.Unlock()

	nodes := CoveredOutbounds(h.router, h.tags)
	tags := make(map[string]struct{})
	for _, node := range nodes {
		tag := node.Tag()
		tags[tag] = struct{}{}
		// make it known to the health check results
		r, ok := h.results[tag]
		if !ok {
			// validity is 2 times to sampling period, since the check are
			// distributed in the time line randomly, in extreme cases,
			// previous checks are distributed on the left, and latters
			// on the right
			validity := time.Duration(h.options.Interval) * time.Duration(h.options.SamplingCount) * 2
			r = newRTTStorage(h.options.SamplingCount, validity)
			h.results[tag] = r
		}
	}
	// remove unused rttStorage
	for tag := range h.results {
		if _, ok := tags[tag]; !ok {
			delete(h.results, tag)
		}
	}
	return nodes
}
