package option

type WSCInboundOptions struct {
	ListenOptions
}

type WSCOutboundOptions struct {
	DialerOptions
	ServerOptions
	OutboundTLSOptionsContainer
	Network NetworkList `json:"network,omitempty"`
	Auth    string      `json:"auth"`
	Path    string      `json:"path"`
}
