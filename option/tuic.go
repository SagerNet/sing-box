package option

type TUICInboundOptions struct {
	ListenOptions
	Users             []TUICUser         `json:"users,omitempty"`
	CongestionControl string             `json:"congestion_control,omitempty"`
	AuthTimeout       Duration           `json:"auth_timeout,omitempty"`
	ZeroRTTHandshake  bool               `json:"zero_rtt_handshake,omitempty"`
	Heartbeat         Duration           `json:"heartbeat,omitempty"`
	TLS               *InboundTLSOptions `json:"tls,omitempty"`
}

type TUICUser struct {
	Name     string `json:"name,omitempty"`
	UUID     string `json:"uuid,omitempty"`
	Password string `json:"password,omitempty"`
}

type TUICOutboundOptions struct {
	DialerOptions
	ServerOptions
	UUID              string              `json:"uuid,omitempty"`
	Password          string              `json:"password,omitempty"`
	CongestionControl string              `json:"congestion_control,omitempty"`
	UDPRelayMode      string              `json:"udp_relay_mode,omitempty"`
	ZeroRTTHandshake  bool                `json:"zero_rtt_handshake,omitempty"`
	Heartbeat         Duration            `json:"heartbeat,omitempty"`
	Network           NetworkList         `json:"network,omitempty"`
	TLS               *OutboundTLSOptions `json:"tls,omitempty"`
}
