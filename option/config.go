package option

import "github.com/sagernet/sing/common"

type Options struct {
	Log       *LogOption    `json:"log"`
	Inbounds  []Inbound     `json:"inbounds,omitempty"`
	Outbounds []Outbound    `json:"outbounds,omitempty"`
	Route     *RouteOptions `json:"route,omitempty"`
}

func (o Options) Equals(other Options) bool {
	return common.ComparablePtrEquals(o.Log, other.Log) &&
		common.SliceEquals(o.Inbounds, other.Inbounds) &&
		common.ComparableSliceEquals(o.Outbounds, other.Outbounds) &&
		common.PtrEquals(o.Route, other.Route)
}

type LogOption struct {
	Disabled bool   `json:"disabled,omitempty"`
	Level    string `json:"level,omitempty"`
	Output   string `json:"output,omitempty"`
}
