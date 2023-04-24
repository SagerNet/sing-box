package option

type VLESSInboundOptions struct {
	ListenOptions
	Users     []VLESSUser            `json:"users,omitempty"`
	TLS       *InboundTLSOptions     `json:"tls,omitempty"`
	Transport *V2RayTransportOptions `json:"transport,omitempty"`
}

type VLESSUser struct {
	Name string `json:"name"`
	UUID string `json:"uuid"`
	Flow string `json:"flow,omitempty"`
}

type VLESSOutboundOptions struct {
	DialerOptions
	ServerOptions
	UUID           string                 `json:"uuid"`
	Flow           string                 `json:"flow,omitempty"`
	Network        NetworkList            `json:"network,omitempty"`
	TLS            *OutboundTLSOptions    `json:"tls,omitempty"`
	Multiplex      *MultiplexOptions      `json:"multiplex,omitempty"`
	Transport      *V2RayTransportOptions `json:"transport,omitempty"`
	PacketEncoding *string                `json:"packet_encoding,omitempty"`
}
