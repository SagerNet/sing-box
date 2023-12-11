package option

type Hysteria2InboundOptions struct {
	ListenOptions
	UpMbps                int             `json:"up_mbps,omitempty"`
	DownMbps              int             `json:"down_mbps,omitempty"`
	Obfs                  *Hysteria2Obfs  `json:"obfs,omitempty"`
	Users                 []Hysteria2User `json:"users,omitempty"`
	IgnoreClientBandwidth bool            `json:"ignore_client_bandwidth,omitempty"`
	InboundTLSOptionsContainer
	Masquerade  string `json:"masquerade,omitempty"`
	BrutalDebug bool   `json:"brutal_debug,omitempty"`
}

type Hysteria2Obfs struct {
	Type     string `json:"type,omitempty"`
	Password string `json:"password,omitempty"`
}

type Hysteria2User struct {
	Name     string `json:"name,omitempty"`
	Password string `json:"password,omitempty"`
}

type Hysteria2OutboundOptions struct {
	DialerOptions
	ServerOptions
	UpMbps   int            `json:"up_mbps,omitempty"`
	DownMbps int            `json:"down_mbps,omitempty"`
	Obfs     *Hysteria2Obfs `json:"obfs,omitempty"`
	Password string         `json:"password,omitempty"`
	Network  NetworkList    `json:"network,omitempty"`
	OutboundTLSOptionsContainer
	BrutalDebug bool `json:"brutal_debug,omitempty"`
}
