package constant

// DefaultURLTestBandwidthURLPrefix is the counterpart to the latency probe's
// generate_204 endpoint. The requested byte count is appended from max_bytes, so the
// response is exactly as large as the probe will read and nothing is sent that the
// probe will cancel away.
const DefaultURLTestBandwidthURLPrefix = "https://speed.cloudflare.com/__down?bytes="

// Selection strategies for the urltest outbound group.
const (
	// URLTestStrategyLatency ranks outbounds by the latency probe alone. This is the
	// default, and the only strategy available when bandwidth testing is disabled.
	URLTestStrategyLatency = "latency"
	// URLTestStrategyThroughput ranks outbounds by measured throughput, ignoring
	// latency beyond liveness.
	URLTestStrategyThroughput = "throughput"
	// URLTestStrategyThroughputWithLatencyFloor discards outbounds whose latency
	// exceeds the configured floor, then ranks the survivors by throughput.
	URLTestStrategyThroughputWithLatencyFloor = "throughput_with_latency_floor"
)
