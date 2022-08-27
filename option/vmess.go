package option

type VMessInboundOptions struct {
	ListenOptions
	Users     []VMessUser            `json:"users,omitempty"`
	TLS       *InboundTLSOptions     `json:"tls,omitempty"`
	Transport *V2RayTransportOptions `json:"transport,omitempty"`
}

type VMessUser struct {
	Name    string `json:"name"`
	UUID    string `json:"uuid"`
	AlterId int    `json:"alterId,omitempty"`
}

type VMessOutboundOptions struct {
	OutboundDialerOptions
	ServerOptions
	UUID                string                 `json:"uuid"`
	Security            string                 `json:"security"`
	AlterId             int                    `json:"alter_id,omitempty"`
	GlobalPadding       bool                   `json:"global_padding,omitempty"`
	AuthenticatedLength bool                   `json:"authenticated_length,omitempty"`
	Network             NetworkList            `json:"network,omitempty"`
	TLS                 *OutboundTLSOptions    `json:"tls,omitempty"`
	PacketAddr          bool                   `json:"packet_addr,omitempty"`
	Multiplex           *MultiplexOptions      `json:"multiplex,omitempty"`
	Transport           *V2RayTransportOptions `json:"transport,omitempty"`
}
