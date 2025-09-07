package option

type WSCUsageReport struct {
	Traffic int64    `json:"traffic,omitempty"`
	Time    Duration `json:"time,omitempty"`
}

type WSCRule struct {
	Action    string        `json:"action"`
	Direction string        `json:"direction,omitempty"`
	Args      []interface{} `json:"args"`
}

type WSCInboundOptions struct {
	ListenOptions
	InboundTLSOptionsContainer
	Multiplex            *InboundMultiplexOptions `json:"multiplex,omitempty"`
	Transport            *V2RayTransportOptions   `json:"transport,omitempty"`
	MaxConnectionPerUser int                      `json:"max_connections,omitempty"`
	UsageTraffic         WSCUsageReport           `json:"usage_traffic,omitempty"`
}

type WSCOutboundOptions struct {
	DialerOptions
	ServerOptions
	OutboundTLSOptionsContainer
	Multiplex *OutboundMultiplexOptions `json:"multiplex,omitempty"`
	Transport *V2RayTransportOptions    `json:"transport,omitempty"`
	Network   NetworkList               `json:"network,omitempty"`
	Auth      string                    `json:"auth"`
	Rules     []WSCRule                 `json:"rules,omitempty"`
}
