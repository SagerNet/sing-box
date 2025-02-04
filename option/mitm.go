package option

import (
	"github.com/sagernet/sing/common/json/badoption"
)

type MITMOptions struct {
	Enabled              bool                  `json:"enabled,omitempty"`
	HTTP2Enabled         bool                  `json:"http2_enabled,omitempty"`
	TLSDecryptionOptions *TLSDecryptionOptions `json:"tls_decryption,omitempty"`
}

type TLSDecryptionOptions struct {
	Enabled     bool   `json:"enabled,omitempty"`
	KeyPair     string `json:"key_pair_p12,omitempty"`
	KeyPassword string `json:"key_password,omitempty"`
}

type MITMRouteOptions struct {
	Enabled            bool                                            `json:"enabled,omitempty"`
	Print              bool                                            `json:"print,omitempty"`
	Script             badoption.Listable[MITMRouteSurgeScriptOptions] `json:"sg_script,omitempty"`
	SurgeURLRewrite    badoption.Listable[SurgeURLRewriteLine]         `json:"sg_url_rewrite,omitempty"`
	SurgeHeaderRewrite badoption.Listable[SurgeHeaderRewriteLine]      `json:"sg_header_rewrite,omitempty"`
	SurgeBodyRewrite   badoption.Listable[SurgeBodyRewriteLine]        `json:"sg_body_rewrite,omitempty"`
	SurgeMapLocal      badoption.Listable[SurgeMapLocalLine]           `json:"sg_map_local,omitempty"`
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
