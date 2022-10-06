package option

type ShadowTLSInboundOptions struct {
	ListenOptions
	Version   int                       `json:"version,omitempty"`
	Password  string                    `json:"password,omitempty"`
	Handshake ShadowTLSHandshakeOptions `json:"handshake"`
}

type ShadowTLSHandshakeOptions struct {
	ServerOptions
	DialerOptions
}

type ShadowTLSOutboundOptions struct {
	DialerOptions
	ServerOptions
	Version  int                 `json:"version,omitempty"`
	Password string              `json:"password,omitempty"`
	TLS      *OutboundTLSOptions `json:"tls,omitempty"`
}
