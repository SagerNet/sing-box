package option

import (
	"github.com/sagernet/sing/common/json/badoption"
)

type MITMOptions struct {
	Enabled      bool `json:"enabled,omitempty"`
	HTTP2Enabled bool `json:"http2_enabled,omitempty"`
}

type MITMRouteOptions struct {
	Enabled            bool                                       `json:"enabled,omitempty"`
	Print              bool                                       `json:"print,omitempty"`
	SurgeURLRewrite    badoption.Listable[SurgeURLRewriteLine]    `json:"surge_url_rewrite,omitempty"`
	SurgeHeaderRewrite badoption.Listable[SurgeHeaderRewriteLine] `json:"surge_header_rewrite,omitempty"`
	SurgeBodyRewrite   badoption.Listable[SurgeBodyRewriteLine]   `json:"surge_body_rewrite,omitempty"`
	SurgeMapLocal      badoption.Listable[SurgeMapLocalLine]      `json:"surge_map_local,omitempty"`
}
