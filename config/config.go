package config

type Config struct {
	Log       *LogConfig `json:"log"`
	Inbounds  []Inbound  `json:"inbounds,omitempty"`
	Outbounds []Outbound `json:"outbounds,omitempty"`
	Routes    []Route    `json:"routes,omitempty"`
}

type LogConfig struct {
	Level string `json:"level,omitempty"`
}
