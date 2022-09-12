package option

type VLESSOutboundOptions struct {
	DialerOptions
	ServerOptions
	UUID           string                 `json:"uuid"`
	Network        NetworkList            `json:"network,omitempty"`
	TLS            *OutboundTLSOptions    `json:"tls,omitempty"`
	Transport      *V2RayTransportOptions `json:"transport,omitempty"`
	PacketEncoding string                 `json:"packet_encoding,omitempty"`
}
