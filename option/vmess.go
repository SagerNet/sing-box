package option

type VMessInboundOptions struct {
	ListenOptions
	Users []VMessUser        `json:"users,omitempty"`
	TLS   *InboundTLSOptions `json:"tls,omitempty"`
}

type VMessUser struct {
	Name string `json:"name"`
	UUID string `json:"uuid"`
}

type VMessOutboundOptions struct {
	OutboundDialerOptions
	ServerOptions
	UUID                string              `json:"uuid"`
	Security            string              `json:"security"`
	AlterId             int                 `json:"alter_id,omitempty"`
	GlobalPadding       bool                `json:"global_padding,omitempty"`
	AuthenticatedLength bool                `json:"authenticated_length,omitempty"`
	Network             NetworkList         `json:"network,omitempty"`
	TLSOptions          *OutboundTLSOptions `json:"tls,omitempty"`
}
