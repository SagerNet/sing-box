package option

// SmartOutboundOptions is the Dart Smart group (Surge-inspired connection failover).
// dart-smart:options
type SmartOutboundOptions struct {
	Outbounds                 []string `json:"outbounds"`
	Mode                      string   `json:"mode,omitempty"`
	InterruptExistConnections bool     `json:"interrupt_exist_connections,omitempty"`
}
