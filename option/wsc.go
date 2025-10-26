package option

type WSCInboundOptions struct {
	ListenOptions
	InboundTLSOptionsContainer
	Users []WSCUser `json:"users"`
	Path  string
}

type WSCUser struct {
	Auth string `json:"auth"`
}

type WSCOutboundOptions struct {
	DialerOptions
	ServerOptions
	OutboundTLSOptionsContainer
	Auth string
	Path string
}
