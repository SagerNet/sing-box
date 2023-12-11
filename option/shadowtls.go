package option

type ShadowTLSInboundOptions struct {
	ListenOptions
	Version                int                                  `json:"version,omitempty"`
	Password               string                               `json:"password,omitempty"`
	Users                  []ShadowTLSUser                      `json:"users,omitempty"`
	Handshake              ShadowTLSHandshakeOptions            `json:"handshake,omitempty"`
	HandshakeForServerName map[string]ShadowTLSHandshakeOptions `json:"handshake_for_server_name,omitempty"`
	StrictMode             bool                                 `json:"strict_mode,omitempty"`
}

type ShadowTLSUser struct {
	Name     string `json:"name,omitempty"`
	Password string `json:"password,omitempty"`
}

type ShadowTLSHandshakeOptions struct {
	ServerOptions
	DialerOptions
}

type ShadowTLSOutboundOptions struct {
	DialerOptions
	ServerOptions
	Version  int    `json:"version,omitempty"`
	Password string `json:"password,omitempty"`
	OutboundTLSOptionsContainer
}
