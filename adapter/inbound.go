package adapter

import (
	M "github.com/sagernet/sing/common/metadata"
)

type Inbound interface {
	Service
	Type() string
	Tag() string
}

type InboundContext struct {
	Inbound     string
	Network     string
	Source      M.Socksaddr
	Destination M.Socksaddr
	Domain      string
	Protocol    string

	// cache

	SourceGeoIPCode string
	GeoIPCode       string
	// ProcessPath     string
}
