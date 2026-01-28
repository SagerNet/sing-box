package option

import (
	"github.com/sagernet/sing/common/auth"
)

type TrustTunnelInboundOptions struct {
	ListenOptions
	Users                 []auth.User `json:"users,omitempty"`
	QUICCongestionControl string      `json:"quic_congestion_control,omitempty"`
	Network               NetworkList `json:"network,omitempty"`
	InboundTLSOptionsContainer
}

type TrustTunnelOutboundOptions struct {
	DialerOptions
	ServerOptions
	Username              string `json:"username,omitempty"`
	Password              string `json:"password,omitempty"`
	HealthCheck           bool   `json:"health_check,omitempty"`
	QUIC                  bool   `json:"quic,omitempty"`
	QUICCongestionControl string `json:"quic_congestion_control,omitempty"`
	OutboundTLSOptionsContainer
}
