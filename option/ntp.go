package option

type NTPOptions struct {
	Enabled       bool     `json:"enabled,omitempty"`
	Server        string   `json:"server,omitempty"`
	ServerPort    uint16   `json:"server_port,omitempty"`
	Interval      Duration `json:"interval,omitempty"`
	WriteToSystem bool     `json:"write_to_system,omitempty"`
	DialerOptions
}
