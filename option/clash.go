package option

type ClashAPIOptions struct {
	ExternalController string `json:"external_controller,omitempty"`
	ExternalUI         string `json:"external_ui,omitempty"`
	Secret             string `json:"secret,omitempty"`

	DefaultMode   string `json:"default_mode,omitempty"`
	StoreSelected bool   `json:"store_selected,omitempty"`
	CacheFile     string `json:"cache_file,omitempty"`
}

type SelectorOutboundOptions struct {
	Outbounds []string `json:"outbounds"`
	Default   string   `json:"default,omitempty"`
}
