package option

type TunPlatformOptions struct {
	HTTPProxy *HTTPProxyOptions `json:"http_proxy,omitempty"`
}

type HTTPProxyOptions struct {
	Enabled bool `json:"enabled,omitempty"`
	ServerOptions
	BypassDomain Listable[string] `json:"bypass_domain,omitempty"`
	MatchDomain  Listable[string] `json:"match_domain,omitempty"`
}
