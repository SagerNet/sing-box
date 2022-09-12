package option

type ProviderOutboundOptions struct {
	Url    string `json:"url"`
	Filter string `json:"filter"`
	Interval  int `json:"interval,omitempty"`
}
