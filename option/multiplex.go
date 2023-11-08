package option

type InboundMultiplexOptions struct {
	Enabled bool           `json:"enabled,omitempty"`
	Padding bool           `json:"padding,omitempty"`
	Brutal  *BrutalOptions `json:"brutal,omitempty"`
}

type OutboundMultiplexOptions struct {
	Enabled        bool           `json:"enabled,omitempty"`
	Protocol       string         `json:"protocol,omitempty"`
	MaxConnections int            `json:"max_connections,omitempty"`
	MinStreams     int            `json:"min_streams,omitempty"`
	MaxStreams     int            `json:"max_streams,omitempty"`
	Padding        bool           `json:"padding,omitempty"`
	Brutal         *BrutalOptions `json:"brutal,omitempty"`
}

type BrutalOptions struct {
	Enabled  bool `json:"enabled,omitempty"`
	UpMbps   int  `json:"up_mbps,omitempty"`
	DownMbps int  `json:"down_mbps,omitempty"`
}
