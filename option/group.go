package option

import (
	"encoding/json"
	"fmt"

	"github.com/sagernet/sing/common/json/badoption"
)

type SelectorOutboundOptions struct {
	Outbounds                 []string `json:"outbounds"`
	Default                   string   `json:"default,omitempty"`
	InterruptExistConnections bool     `json:"interrupt_exist_connections,omitempty"`
}

// URLTestMode declares the selection strategy for URLTest outbound groups.
// Valid values (JSON):
//   - "min_latency" (default): pick the outbound with the lowest measured delay (with tolerance)
//   - "first_available": pick the first outbound (by order) that is currently healthy (has a recent successful test)
//
// If the JSON field is empty, MinLatency is used as default.
type URLTestMode string

const (
	MinLatency     URLTestMode = "min_latency"
	FirstAvailable URLTestMode = "first_available"
)

// UnmarshalJSON validates URLTestMode and fills a sensible default when omitted.
func (m *URLTestMode) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if s == "" {
		*m = MinLatency
		return nil
	}
	switch URLTestMode(s) {
	case MinLatency, FirstAvailable:
		*m = URLTestMode(s)
		return nil
	default:
		return fmt.Errorf("invalid select_mode: %q", s)
	}
}

type URLTestOutboundOptions struct {
	Outbounds                 []string           `json:"outbounds"`
	URL                       string             `json:"url,omitempty"`
	Interval                  badoption.Duration `json:"interval,omitempty"`
	Tolerance                 uint16             `json:"tolerance,omitempty"`
	IdleTimeout               badoption.Duration `json:"idle_timeout,omitempty"`
	InterruptExistConnections bool               `json:"interrupt_exist_connections,omitempty"`
	SelectMode                URLTestMode        `json:"select_mode,omitempty"`
}
