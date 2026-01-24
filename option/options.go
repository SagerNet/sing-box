package option

import (
	"bytes"
	"context"

	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/json"
)

type _Options struct {
	RawMessage   json.RawMessage      `json:"-"`
	Schema       string               `json:"$schema,omitempty"`
	Log          *LogOptions          `json:"log,omitempty"`
	DNS          *DNSOptions          `json:"dns,omitempty"`
	NTP          *NTPOptions          `json:"ntp,omitempty"`
	Certificate  *CertificateOptions  `json:"certificate,omitempty"`
	Endpoints    []Endpoint           `json:"endpoints,omitempty"`
	Inbounds     []Inbound            `json:"inbounds,omitempty"`
	Outbounds    []Outbound           `json:"outbounds,omitempty"`
	Route        *RouteOptions        `json:"route,omitempty"`
	Services     []Service            `json:"services,omitempty"`
	Experimental *ExperimentalOptions `json:"experimental,omitempty"`
}

type Options _Options

func (o *Options) UnmarshalJSONContext(ctx context.Context, content []byte) error {
	decoder := json.NewDecoderContext(ctx, bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	err := decoder.Decode((*_Options)(o))
	if err != nil {
		return err
	}
	o.RawMessage = content
	return checkOptions(o)
}

type LogOptions struct {
	Disabled     bool   `json:"disabled,omitempty"`
	Level        string `json:"level,omitempty"`
	Output       string `json:"output,omitempty"`
	Timestamp    bool   `json:"timestamp,omitempty"`
	DisableColor bool   `json:"-"`
}

type StubOptions struct{}

func checkOptions(options *Options) error {
	err := checkInbounds(options.Inbounds)
	if err != nil {
		return err
	}
	err = checkOutbounds(options.Outbounds, options.Endpoints)
	if err != nil {
		return err
	}
	return nil
}

func checkInbounds(inbounds []Inbound) error {
	seen := make(map[string]bool)
	for i, inbound := range inbounds {
		tag := inbound.Tag
		if tag == "" {
			tag = F.ToString(i)
		}
		if seen[tag] {
			return E.New("duplicate inbound tag: ", tag)
		}
		seen[tag] = true
	}
	return nil
}

func checkOutbounds(outbounds []Outbound, endpoints []Endpoint) error {
	seen := make(map[string]bool)
	for i, outbound := range outbounds {
		tag := outbound.Tag
		if tag == "" {
			tag = F.ToString(i)
		}
		if seen[tag] {
			return E.New("duplicate outbound/endpoint tag: ", tag)
		}
		seen[tag] = true
	}
	for i, endpoint := range endpoints {
		tag := endpoint.Tag
		if tag == "" {
			tag = F.ToString(i)
		}
		if seen[tag] {
			return E.New("duplicate outbound/endpoint tag: ", tag)
		}
		seen[tag] = true
	}
	return nil
}
