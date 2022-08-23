package option

type DirectInboundOptions struct {
	ListenOptions
	Network         NetworkList `json:"network,omitempty"`
	OverrideAddress string      `json:"override_address,omitempty"`
	OverridePort    uint16      `json:"override_port,omitempty"`
}

type DirectOutboundOptions struct {
	OutboundDialerOptions
	OverrideAddress string `json:"override_address,omitempty"`
	OverridePort    uint16 `json:"override_port,omitempty"`
	ProxyProtocol   uint8  `json:"proxy_protocol,omitempty"`
}
