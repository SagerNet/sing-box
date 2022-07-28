package option

type ClashAPIOptions struct {
	ExternalController string `json:"external_controller,omitempty"`
	ExternalUI         string `json:"external_ui,omitempty"`
	Secret             string `json:"secret,omitempty"`
}

type SelectorOutboundOptions struct {
	Outbounds []string `json:"outbounds"`
	Default   string   `json:"default,omitempty"`
}
