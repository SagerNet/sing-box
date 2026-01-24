package option

import "github.com/sagernet/sing/common/json/badoption"

type TunPlatformOptions struct {
	HTTPProxy *HTTPProxyOptions `json:"http_proxy,omitempty"`
}

type HTTPProxyOptions struct {
	Enabled bool `json:"enabled,omitempty"`
	ServerOptions
	BypassDomain badoption.Listable[string] `json:"bypass_domain,omitempty"`
	MatchDomain  badoption.Listable[string] `json:"match_domain,omitempty"`
}
