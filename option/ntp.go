package option

import "github.com/sagernet/sing/common/json/badoption"

type NTPOptions struct {
	Enabled       bool               `json:"enabled,omitempty"`
	Interval      badoption.Duration `json:"interval,omitempty"`
	WriteToSystem bool               `json:"write_to_system,omitempty"`
	ServerOptions
	DialerOptions
}
