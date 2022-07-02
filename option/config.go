package option

type Options struct {
	Log       *LogOption `json:"log"`
	Inbounds  []Inbound  `json:"inbounds,omitempty"`
	Outbounds []Outbound `json:"outbounds,omitempty"`
	Routes    []Rule     `json:"routes,omitempty"`
}

type LogOption struct {
	Disabled bool   `json:"disabled,omitempty"`
	Level    string `json:"level,omitempty"`
	Output   string `json:"output,omitempty"`
}
