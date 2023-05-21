package option

type BadsocksInboundOptions struct {
	ListenOptions
	Password string `json:"password,omitempty"`
}

type BadsocksOutboundOptions struct {
	DialerOptions
	ServerOptions
	Password         string            `json:"password"`
	MultiplexOptions *MultiplexOptions `json:"multiplex,omitempty"`
}
