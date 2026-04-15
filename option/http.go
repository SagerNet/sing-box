package option

import (
	"reflect"

	"github.com/sagernet/sing/common/byteformats"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
)

type HTTP2Options struct {
	IdleTimeout             badoption.Duration      `json:"idle_timeout,omitempty"`
	KeepAlivePeriod         badoption.Duration      `json:"keep_alive_period,omitempty"`
	StreamReceiveWindow     byteformats.MemoryBytes `json:"stream_receive_window,omitempty"`
	ConnectionReceiveWindow byteformats.MemoryBytes `json:"connection_receive_window,omitempty"`
	MaxConcurrentStreams    int                     `json:"max_concurrent_streams,omitempty"`
}

type QUICOptions struct {
	HTTP2Options
	InitialPacketSize       int  `json:"initial_packet_size,omitempty"`
	DisablePathMTUDiscovery bool `json:"disable_path_mtu_discovery,omitempty"`
}

type _HTTPClientOptions struct {
	Tag                     string               `json:"tag,omitempty"`
	Engine                  string               `json:"engine,omitempty"`
	Version                 int                  `json:"version,omitempty"`
	DisableVersionFallback  bool                 `json:"disable_version_fallback,omitempty"`
	Headers                 badoption.HTTPHeader `json:"headers,omitempty"`
	HTTP2Options            HTTP2Options         `json:"-"`
	HTTP3Options            QUICOptions          `json:"-"`
	DefaultOutbound         bool                 `json:"-"`
	DisableEmptyDirectCheck bool                 `json:"-"`
	ResolveOnDetour         bool                 `json:"-"`
	DirectResolver          bool                 `json:"-"`
	OutboundTLSOptionsContainer
	DialerOptions
}

type (
	HTTPClient        _HTTPClientOptions
	HTTPClientOptions _HTTPClientOptions
)

func (h HTTPClient) Options() HTTPClientOptions {
	options := HTTPClientOptions(h)
	options.Tag = ""
	return options
}

func (o HTTPClientOptions) IsEmpty() bool {
	if o.Tag != "" {
		return false
	}
	o.DefaultOutbound = false
	o.ResolveOnDetour = false
	o.DirectResolver = false
	return reflect.ValueOf(_HTTPClientOptions(o)).IsZero()
}

func (o HTTPClientOptions) MarshalJSON() ([]byte, error) {
	if o.Tag != "" {
		return json.Marshal(o.Tag)
	}
	return badjson.MarshallObjects(_HTTPClientOptions(o), httpClientVariant(_HTTPClientOptions(o)))
}

func (o *HTTPClientOptions) UnmarshalJSON(content []byte) error {
	if len(content) > 0 && content[0] == '"' {
		*o = HTTPClientOptions{}
		return json.Unmarshal(content, &o.Tag)
	}
	var options _HTTPClientOptions
	err := json.Unmarshal(content, &options)
	if err != nil {
		return err
	}
	err = unmarshalHTTPClientVersionOptions(content, &options, &options)
	if err != nil {
		return err
	}
	options.Tag = ""
	*o = HTTPClientOptions(options)
	return nil
}

func (h HTTPClient) MarshalJSON() ([]byte, error) {
	return badjson.MarshallObjects(_HTTPClientOptions(h), httpClientVariant(_HTTPClientOptions(h)))
}

func (h *HTTPClient) UnmarshalJSON(content []byte) error {
	err := json.Unmarshal(content, (*_HTTPClientOptions)(h))
	if err != nil {
		return err
	}
	return unmarshalHTTPClientVersionOptions(content, (*_HTTPClientOptions)(h), (*_HTTPClientOptions)(h))
}

func unmarshalHTTPClientVersionOptions(content []byte, baseStruct any, options *_HTTPClientOptions) error {
	switch options.Version {
	case 1:
		return json.UnmarshalDisallowUnknownFields(content, baseStruct)
	case 0, 2:
		options.Version = 2
		return badjson.UnmarshallExcluded(content, baseStruct, &options.HTTP2Options)
	case 3:
		return badjson.UnmarshallExcluded(content, baseStruct, &options.HTTP3Options)
	default:
		return E.New("unknown HTTP version: ", options.Version)
	}
}

func httpClientVariant(options _HTTPClientOptions) any {
	switch options.Version {
	case 1:
		return nil
	case 0, 2:
		return options.HTTP2Options
	case 3:
		return options.HTTP3Options
	default:
		return nil
	}
}
