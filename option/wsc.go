package option

type WSCInboundOptions struct {
	ListenOptions
}

type WSCOutboundOptions struct {
	DialerOptions
	ServerOptions
	Network NetworkList `json:"network,omitempty"`
}
