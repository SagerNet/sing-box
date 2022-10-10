package option

// LeastLoadOutboundOptions is the options for leastload outbound
type LeastLoadOutboundOptions struct {
	Outbounds []string `json:"outbounds"`
	Fallback  string   `json:"fallback,omitempty"`
	// health check settings
	HealthCheck *HealthCheckSettings `json:"healthCheck,omitempty"`
	// cost settings
	Costs []*StrategyWeight `json:"costs,omitempty"`
	// ping rtt baselines (ms)
	Baselines []Duration `json:"baselines,omitempty"`
	// expected nodes count to select
	Expected int32 `json:"expected,omitempty"`
	// max acceptable rtt (ms), filter away high delay nodes. defalut 0
	MaxRTT Duration `json:"maxRTT,omitempty"`
	// acceptable failure rate
	Tolerance float64 `json:"tolerance,omitempty"`
}

// HealthCheckSettings is the settings for health check
type HealthCheckSettings struct {
	Destination   string   `json:"destination"`
	Connectivity  string   `json:"connectivity"`
	Interval      Duration `json:"interval"`
	SamplingCount int      `json:"sampling"`
	Timeout       Duration `json:"timeout"`
}

// StrategyWeight is the weight for a balancing strategy
type StrategyWeight struct {
	Regexp bool    `json:"regexp,omitempty"`
	Match  string  `json:"match,omitempty"`
	Value  float32 `json:"value,omitempty"`
}
