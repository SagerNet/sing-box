package option

type SubscriptionServiceOptions struct {
	Interval       Duration                      `json:"interval,omitempty"`
	DownloadDetour string                        `json:"download_detour,omitempty"`
	Providers      []SubscriptionProviderOptions `json:"providers"`

	DialerOptions
}

type SubscriptionProviderOptions struct {
	Tag      string   `json:"tag,omitempty"`
	URL      string   `json:"url"`
	Excludes []string `json:"excludes,omitempty"`
}
