package option

import (
	"bytes"

	"github.com/sagernet/sing/common"

	"github.com/goccy/go-json"
)

type _Options struct {
	Log       *LogOption    `json:"log,omitempty"`
	Inbounds  []Inbound     `json:"inbounds,omitempty"`
	Outbounds []Outbound    `json:"outbounds,omitempty"`
	Route     *RouteOptions `json:"route,omitempty"`
}

type Options _Options

func (o *Options) UnmarshalJSON(content []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	return decoder.Decode((*_Options)(o))
}

func (o Options) Equals(other Options) bool {
	return common.ComparablePtrEquals(o.Log, other.Log) &&
		common.SliceEquals(o.Inbounds, other.Inbounds) &&
		common.ComparableSliceEquals(o.Outbounds, other.Outbounds) &&
		common.PtrEquals(o.Route, other.Route)
}

type LogOption struct {
	Disabled     bool   `json:"disabled,omitempty"`
	Level        string `json:"level,omitempty"`
	Output       string `json:"output,omitempty"`
	Timestamp    bool   `json:"timestamp,omitempty"`
	DisableColor bool   `json:"-"`
}
