package option

type ClashAPIOptions struct {
	DefaultMode        string `json:"default_mode,omitempty"`
	ExternalController string `json:"external_controller,omitempty"`
	ExternalUI         string `json:"external_ui,omitempty"`
	Secret             string `json:"secret,omitempty"`
}

type SelectorOutboundOptions struct {
	Outbounds []string `json:"outbounds"`
	Default   string   `json:"default,omitempty"`
}
