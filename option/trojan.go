package option

type TrojanInboundOptions struct {
	ListenOptions
	Users     []TrojanUser           `json:"users,omitempty"`
	TLS       *InboundTLSOptions     `json:"tls,omitempty"`
	Fallback  *ServerOptions         `json:"fallback,omitempty"`
	Transport *V2RayTransportOptions `json:"transport,omitempty"`
}

type TrojanUser struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type TrojanOutboundOptions struct {
	OutboundDialerOptions
	ServerOptions
	Password  string                 `json:"password"`
	Network   NetworkList            `json:"network,omitempty"`
	TLS       *OutboundTLSOptions    `json:"tls,omitempty"`
	Multiplex *MultiplexOptions      `json:"multiplex,omitempty"`
	Transport *V2RayTransportOptions `json:"transport,omitempty"`
}
