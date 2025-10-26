package option

type WSCInboundOptions struct {
	ListenOptions
	Users []WSCUser `json:"users"`
	InboundTLSOptionsContainer
}

type WSCUser struct {
	Auth string `json:"auth"`
}

type WSCOutboundOptions struct {
	DialerOptions
	ServerOptions
	OutboundTLSOptionsContainer
	Auth string
}
