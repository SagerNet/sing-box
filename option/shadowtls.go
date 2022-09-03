package option

type ShadowTLSInboundOptions struct {
	ListenOptions
	Handshake ShadowTLSHandshakeOptions `json:"handshake"`
}

type ShadowTLSHandshakeOptions struct {
	ServerOptions
	DialerOptions
}

type ShadowTLSOutboundOptions struct {
	DialerOptions
	ServerOptions
	TLS *OutboundTLSOptions `json:"tls,omitempty"`
}
