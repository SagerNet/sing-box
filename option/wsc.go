package option

type WSCInboundOptions struct {
	ListenOptions
}

type WSCOutboundOptions struct {
	DialerOptions
	Network NetworkList `json:"network,omitempty"`
	Auth    string      `json:"auth"`
	Host    string      `json:"host"`
	Path    string      `json:"path"`
}
