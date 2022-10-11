package option

// LeastPingOutboundOptions is the options for leastping outbound
type LeastPingOutboundOptions struct {
	BalancerOutboundOptions
	// health check settings
	HealthCheck HealthCheckSettings `json:"health_check,omitempty"`
}

// LeastLoadOutboundOptions is the options for leastload outbound
type LeastLoadOutboundOptions struct {
	BalancerOutboundOptions
	// health check settings
	HealthCheck HealthCheckSettings `json:"health_check,omitempty"`
	// expected nodes count to select
	Expected int32 `json:"expected,omitempty"`
	// ping rtt baselines (ms)
	Baselines []Duration `json:"baselines,omitempty"`
	// cost settings
	Costs []*StrategyWeight `json:"costs,omitempty"`
}

// BalancerOutboundOptions is the options for balancer outbound
type BalancerOutboundOptions struct {
	Outbounds []string `json:"outbounds"`
	Fallback  string   `json:"fallback,omitempty"`
}

// HealthCheckSettings is the settings for health check
type HealthCheckSettings struct {
	Destination   string   `json:"destination"`
	Connectivity  string   `json:"connectivity"`
	Interval      Duration `json:"interval"`
	SamplingCount int      `json:"sampling"`
	Timeout       Duration `json:"timeout"`
	// max acceptable rtt (ms), filter away high delay nodes. defalut 0
	MaxRTT Duration `json:"max_rtt,omitempty"`
	// acceptable failure rate
	Tolerance float64 `json:"tolerance,omitempty"`
}

// StrategyWeight is the weight for a balancing strategy
type StrategyWeight struct {
	Regexp bool    `json:"regexp,omitempty"`
	Match  string  `json:"match,omitempty"`
	Value  float32 `json:"value,omitempty"`
}
