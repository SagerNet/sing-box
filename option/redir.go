package option

type RedirectInboundOptions struct {
	ListenOptions
	AutoRedirect *AutoRedirectOptions `json:"auto_redirect,omitempty"`
}

type AutoRedirectOptions struct {
	Enabled                bool `json:"enabled,omitempty"`
	ContinueOnNoPermission bool `json:"continue_on_no_permission,omitempty"`
}

type TProxyInboundOptions struct {
	ListenOptions
	Network NetworkList `json:"network,omitempty"`
}
