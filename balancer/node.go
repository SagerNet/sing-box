package balancer

var healthPingStatsUntested = RTTStats{
	All:       0,
	Fail:      0,
	Deviation: rttUntested,
	Average:   rttUntested,
	Max:       rttUntested,
	Min:       rttUntested,
}

// Node is a banalcer node with health check result
type Node struct {
	Tag string
	RTTStats
}
