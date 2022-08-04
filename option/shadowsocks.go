package option

type ShadowsocksInboundOptions struct {
	ListenOptions
	Network         NetworkList              `json:"network,omitempty"`
	Method          string                   `json:"method"`
	Password        string                   `json:"password"`
	ControlPassword string                   `json:"control_password,omitempty"`
	Users           []ShadowsocksUser        `json:"users,omitempty"`
	Destinations    []ShadowsocksDestination `json:"destinations,omitempty"`
}

type ShadowsocksUser struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type ShadowsocksDestination struct {
	Name     string `json:"name"`
	Password string `json:"password"`
	ServerOptions
}

type ShadowsocksOutboundOptions struct {
	OutboundDialerOptions
	ServerOptions
	Method           string            `json:"method"`
	Password         string            `json:"password"`
	Network          NetworkList       `json:"network,omitempty"`
	MultiplexOptions *MultiplexOptions `json:"multiplex,omitempty"`
}
