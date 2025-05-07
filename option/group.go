package option

import "github.com/sagernet/sing/common/json/badoption"

type SelectorOutboundOptions struct {
	Outbounds                 []string `json:"outbounds"`
	Default                   string   `json:"default,omitempty"`
	InterruptExistConnections bool     `json:"interrupt_exist_connections,omitempty"`
}

type URLTestOutboundOptions struct {
	Outbounds                 []string           `json:"outbounds"`
	URL                       string             `json:"url,omitempty"`
	Interval                  badoption.Duration `json:"interval,omitempty"`
	Tolerance                 uint16             `json:"tolerance,omitempty"`
	IdleTimeout               badoption.Duration `json:"idle_timeout,omitempty"`
	InterruptExistConnections bool               `json:"interrupt_exist_connections,omitempty"`
}

type BalancerOutboundOptions struct {
	Outbounds     []string           `json:"outbounds"`
	URL           string             `json:"url,omitempty"`
	Interval      badoption.Duration `json:"interval,omitempty"`
	HistoryTTL    badoption.Duration `json:"history_ttl,omitempty"`
	ForceRandom   bool               `json:"force_random,omitempty"`
	RetryCount    int                `json:"retry_count,omitempty"`
	RetryInterval badoption.Duration `json:"retry_interval,omitempty"`
}
