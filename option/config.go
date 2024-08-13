package option

import (
	"bytes"

	"github.com/sagernet/sing/common/json"
)

type _Options struct {
	RawMessage        json.RawMessage      `json:"-"`
	Schema            string               `json:"$schema,omitempty"`
	Log               *LogOptions          `json:"log,omitempty"`
	DNS               *DNSOptions          `json:"dns,omitempty"`
	NTP               *NTPOptions          `json:"ntp,omitempty"`
	Inbounds          []Inbound            `json:"inbounds,omitempty"`
	Outbounds         []Outbound           `json:"outbounds,omitempty"`
	Route             *RouteOptions        `json:"route,omitempty"`
	OutboundProviders []OutboundProvider   `json:"outbound_providers,omitempty"`
	Experimental      *ExperimentalOptions `json:"experimental,omitempty"`
}

type Options _Options

func (o *Options) UnmarshalJSON(content []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	err := decoder.Decode((*_Options)(o))
	if err != nil {
		return err
	}
	o.RawMessage = content
	return nil
}

type LogOptions struct {
	Disabled     bool   `json:"disabled,omitempty"`
	Level        string `json:"level,omitempty"`
	Output       string `json:"output,omitempty"`
	Timestamp    bool   `json:"timestamp,omitempty"`
	DisableColor bool   `json:"-"`
}

type _OutboundProviderOptions struct {
	Outbounds []Outbound `json:"outbounds"`
}

type OutboundProviderOptions _OutboundProviderOptions

func (o *OutboundProviderOptions) UnmarshalJSON(content []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(content))
	err := decoder.Decode((*_OutboundProviderOptions)(o))
	if err != nil {
		return err
	}
	return nil
}
