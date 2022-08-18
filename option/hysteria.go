package option

type HysteriaOutboundOptions struct {
	OutboundDialerOptions
	ServerOptions
	Protocol            string      `json:"protocol"`
	Up                  string      `json:"up"`
	UpMbps              int         `json:"up_mbps"`
	Down                string      `json:"down"`
	DownMbps            int         `json:"down_mbps"`
	Obfs                string      `json:"obfs"`
	Auth                []byte      `json:"auth"`
	AuthString          string      `json:"auth_str"`
	ALPN                string      `json:"alpn"`
	ServerName          string      `json:"server_name"`
	Insecure            bool        `json:"insecure"`
	CustomCA            string      `json:"ca"`
	CustomCAStr         string      `json:"ca_str"`
	ReceiveWindowConn   uint64      `json:"recv_window_conn"`
	ReceiveWindow       uint64      `json:"recv_window"`
	DisableMTUDiscovery bool        `json:"disable_mtu_discovery"`
	Network             NetworkList `json:"network,omitempty"`
}
