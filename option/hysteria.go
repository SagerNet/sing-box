package option

type HysteriaInboundOptions struct {
	ListenOptions
	Up                  string             `json:"up,omitempty"`
	UpMbps              int                `json:"up_mbps,omitempty"`
	Down                string             `json:"down,omitempty"`
	DownMbps            int                `json:"down_mbps,omitempty"`
	Obfs                string             `json:"obfs,omitempty"`
	Auth                []byte             `json:"auth,omitempty"`
	AuthString          string             `json:"auth_str,omitempty"`
	ReceiveWindowConn   uint64             `json:"recv_window_conn,omitempty"`
	ReceiveWindowClient uint64             `json:"recv_window_client,omitempty"`
	MaxConnClient       int                `json:"max_conn_client,omitempty"`
	DisableMTUDiscovery bool               `json:"disable_mtu_discovery,omitempty"`
	TLS                 *InboundTLSOptions `json:"tls,omitempty"`
}

type HysteriaOutboundOptions struct {
	OutboundDialerOptions
	ServerOptions
	Up                  string              `json:"up,omitempty"`
	UpMbps              int                 `json:"up_mbps,omitempty"`
	Down                string              `json:"down,omitempty"`
	DownMbps            int                 `json:"down_mbps,omitempty"`
	Obfs                string              `json:"obfs,omitempty"`
	Auth                []byte              `json:"auth,omitempty"`
	AuthString          string              `json:"auth_str,omitempty"`
	ReceiveWindowConn   uint64              `json:"recv_window_conn,omitempty"`
	ReceiveWindow       uint64              `json:"recv_window,omitempty"`
	DisableMTUDiscovery bool                `json:"disable_mtu_discovery,omitempty"`
	Network             NetworkList         `json:"network,omitempty"`
	TLS                 *OutboundTLSOptions `json:"tls,omitempty"`
}
