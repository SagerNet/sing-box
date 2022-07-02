package option

type Options struct {
	Log       *LogOption    `json:"log"`
	Inbounds  []Inbound     `json:"inbounds,omitempty"`
	Outbounds []Outbound    `json:"outbounds,omitempty"`
	Route     *RouteOptions `json:"route,omitempty"`
}

type LogOption struct {
	Disabled bool   `json:"disabled,omitempty"`
	Level    string `json:"level,omitempty"`
	Output   string `json:"output,omitempty"`
}
