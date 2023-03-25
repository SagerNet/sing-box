package option

type ClashAPIOptions struct {
	ExternalController string `json:"external_controller,omitempty"`
	ExternalUI         string `json:"external_ui,omitempty"`
	Secret             string `json:"secret,omitempty"`
	DefaultMode        string `json:"default_mode,omitempty"`
	StoreSelected      bool   `json:"store_selected,omitempty"`
	StoreFakeIP        bool   `json:"store_fakeip,omitempty"`
	CacheFile          string `json:"cache_file,omitempty"`
}

type SelectorOutboundOptions struct {
	Outbounds []string `json:"outbounds"`
	Default   string   `json:"default,omitempty"`
}

type URLTestOutboundOptions struct {
	Outbounds []string `json:"outbounds"`
	URL       string   `json:"url,omitempty"`
	Interval  Duration `json:"interval,omitempty"`
	Tolerance uint16   `json:"tolerance,omitempty"`
}
