package option

type WSCUsageReport struct {
	Traffic int64    `json:"traffic,omitempty"`
	Time    Duration `json:"time,omitempty"`
}

type WSCRule struct {
	Action string        `json:"action"`
	Args   []interface{} `json:"args"`
}

type WSCInboundOptions struct {
	ListenOptions
	InboundTLSOptionsContainer
	MaxConnectionPerUser int            `json:"max_connections,omitempty"`
	UsageTraffic         WSCUsageReport `json:"usage_traffic,omitempty"`
}

type WSCOutboundOptions struct {
	DialerOptions
	ServerOptions
	OutboundTLSOptionsContainer
	Network NetworkList `json:"network,omitempty"`
	Auth    string      `json:"auth"`
	Path    string      `json:"path"`
	Rules   []WSCRule   `json:"rules,omitempty"`
}
