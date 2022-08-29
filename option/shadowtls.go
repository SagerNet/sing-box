package option

type ShadowTLSInboundOptions struct {
	ListenOptions
	Network         NetworkList `json:"network,omitempty"`
	HandshakeDetour string      `json:"handshake_detour,omitempty"`
}

type ShadowTLSOutboundOptions struct {
	OutboundDialerOptions
	ServerOptions
	TLS *OutboundTLSOptions `json:"tls,omitempty"`
}
