package option

import (
	"github.com/sagernet/sing/common/json/badoption"
)

type MITMOptions struct {
	Enabled      bool `json:"enabled,omitempty"`
	HTTP2Enabled bool `json:"http2_enabled,omitempty"`
}

type MITMRouteOptions struct {
	Enabled            bool                                            `json:"enabled,omitempty"`
	Print              bool                                            `json:"print,omitempty"`
	Script             badoption.Listable[MITMRouteSurgeScriptOptions] `json:"surge_script,omitempty"`
	SurgeURLRewrite    badoption.Listable[SurgeURLRewriteLine]         `json:"surge_url_rewrite,omitempty"`
	SurgeHeaderRewrite badoption.Listable[SurgeHeaderRewriteLine]      `json:"surge_header_rewrite,omitempty"`
	SurgeBodyRewrite   badoption.Listable[SurgeBodyRewriteLine]        `json:"surge_body_rewrite,omitempty"`
	SurgeMapLocal      badoption.Listable[SurgeMapLocalLine]           `json:"surge_map_local,omitempty"`
}

type MITMRouteSurgeScriptOptions struct {
	Tag            string                                `json:"tag"`
	Type           badoption.Listable[string]            `json:"type"`
	Pattern        badoption.Listable[*badoption.Regexp] `json:"pattern"`
	Timeout        badoption.Duration                    `json:"timeout,omitempty"`
	RequiresBody   bool                                  `json:"requires_body,omitempty"`
	MaxSize        int64                                 `json:"max_size,omitempty"`
	BinaryBodyMode bool                                  `json:"binary_body_mode,omitempty"`
	Arguments      badoption.Listable[string]            `json:"arguments,omitempty"`
}
