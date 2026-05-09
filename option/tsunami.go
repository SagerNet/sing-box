package option

type TsunamiOutboundOptions struct {
	DialerOptions
	ServerOptions
	OutboundTLSOptionsContainer
	Password string `json:"password,omitempty"`
}
