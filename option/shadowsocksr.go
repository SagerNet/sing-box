package option

type ShadowsocksROutboundOptions struct {
	DialerOptions
	ServerOptions
	Method        string      `json:"method"`
	Password      string      `json:"password"`
	Obfs          string      `json:"obfs,omitempty"`
	ObfsParam     string      `json:"obfs_param,omitempty"`
	Protocol      string      `json:"protocol,omitempty"`
	ProtocolParam string      `json:"protocol_param,omitempty"`
	Network       NetworkList `json:"network,omitempty"`
}
