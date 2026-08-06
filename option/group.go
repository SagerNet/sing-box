package option

import "github.com/sagernet/sing/common/json/badoption"

type SelectorOutboundOptions struct {
	Outbounds                 []string `json:"outbounds" reference:"outbound"`
	Default                   string   `json:"default,omitempty" reference:"outbound"`
	InterruptExistConnections bool     `json:"interrupt_exist_connections,omitempty"`
}

type URLTestOutboundOptions struct {
	Outbounds                 []string                     `json:"outbounds" reference:"outbound"`
	URL                       string                       `json:"url,omitempty"`
	Interval                  badoption.Duration           `json:"interval,omitempty"`
	Tolerance                 uint16                       `json:"tolerance,omitempty"`
	IdleTimeout               badoption.Duration           `json:"idle_timeout,omitempty"`
	InterruptExistConnections bool                         `json:"interrupt_exist_connections,omitempty"`
	BandwidthTest             *URLTestBandwidthTestOptions `json:"bandwidth_test,omitempty"`
}

// URLTestBandwidthTestOptions configures an optional bandwidth probe that supplements
// the latency probe. It is disabled by default; when disabled, the probe path is
// unchanged and no response body is ever transferred.
type URLTestBandwidthTestOptions struct {
	Enabled bool `json:"enabled,omitempty"`
	// URL must return a body of at least MaxBytes. When empty, a Cloudflare speed
	// endpoint sized to MaxBytes is used; prefer your own, since a shared default is
	// fetched by every client that enables this.
	URL string `json:"url,omitempty"`
	// MaxBytes is a hard cap on the number of body bytes read per probe. It bounds
	// the bytes read, not the memory retained; the reader uses a small fixed buffer.
	MaxBytes            uint32             `json:"max_bytes,omitempty"`
	Timeout             badoption.Duration `json:"timeout,omitempty"`
	Interval            badoption.Duration `json:"interval,omitempty"`
	Concurrency         int                `json:"concurrency,omitempty"`
	Strategy            string             `json:"strategy,omitempty"`
	LatencyFloor        badoption.Duration `json:"latency_floor,omitempty"`
	ThroughputTolerance uint16             `json:"throughput_tolerance,omitempty"`
	Samples             int                `json:"samples,omitempty"`
}
