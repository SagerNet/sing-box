package option

import (
	"bytes"
	"context"

	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/json"
)

type _Options struct {
	RawMessage           json.RawMessage       `json:"-"`
	CommentsSet          *json.CommentSet      `json:"-"`
	Schema               string                `json:"$schema,omitempty"`
	Log                  *LogOptions           `json:"log,omitempty"`
	DNS                  *DNSOptions           `json:"dns,omitempty"`
	NTP                  *NTPOptions           `json:"ntp,omitempty"`
	Certificate          *CertificateOptions   `json:"certificate,omitempty"`
	CertificateProviders []CertificateProvider `json:"certificate_providers,omitempty"`
	HTTPClients          []HTTPClient          `json:"http_clients,omitempty"`
	Endpoints            []Endpoint            `json:"endpoints,omitempty"`
	Inbounds             []Inbound             `json:"inbounds,omitempty"`
	Outbounds            []Outbound            `json:"outbounds,omitempty"`
	Route                *RouteOptions         `json:"route,omitempty"`
	Services             []Service             `json:"services,omitempty"`
	Experimental         *ExperimentalOptions  `json:"experimental,omitempty"`
}

type Options _Options

func (o Options) MarshalJSONContext(ctx context.Context) ([]byte, error) {
	return json.MarshalContext(ctx, (_Options)(o))
}

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

func (o Options) Comments() *json.CommentSet {
	return o.CommentsSet
}

func (o *Options) SetComments(comments *json.CommentSet) {
	o.CommentsSet = comments
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
	err = checkCertificateProviders(options.CertificateProviders)
	if err != nil {
		return err
	}
	err = checkHTTPClients(options.HTTPClients)
	if err != nil {
		return err
	}
	return nil
}

func checkCertificateProviders(providers []CertificateProvider) error {
	seen := make(map[string]bool)
	for i, provider := range providers {
		tag := provider.Tag
		if tag == "" {
			tag = F.ToString(i)
		}
		if seen[tag] {
			return E.New("duplicate certificate provider tag: ", tag)
		}
		seen[tag] = true
	}
	return nil
}

func checkHTTPClients(clients []HTTPClient) error {
	seen := make(map[string]bool)
	for _, client := range clients {
		if client.Tag == "" {
			return E.New("missing http client tag")
		}
		if seen[client.Tag] {
			return E.New("duplicate http client tag: ", client.Tag)
		}
		seen[client.Tag] = true
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
