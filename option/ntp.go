package option

type NTPOptions struct {
	Enabled  bool     `json:"enabled"`
	Interval Duration `json:"interval,omitempty"`
	ServerOptions
	DialerOptions
}
