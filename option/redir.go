package option

type RedirectInboundOptions struct {
	ListenOptions
}

type TProxyInboundOptions struct {
	ListenOptions
	Network      NetworkList    `json:"network,omitempty"`
	UDPMapping   UDPNATBehavior `json:"udp_mapping,omitempty"`
	UDPFiltering UDPNATBehavior `json:"udp_filtering,omitempty"`
	UDPNATMax    uint32         `json:"udp_nat_max,omitempty"`
}
