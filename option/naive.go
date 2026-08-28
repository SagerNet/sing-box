package option

import (
	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/byteformats"
	"github.com/sagernet/sing/common/json/badoption"
)

type NaiveInboundOptions struct {
	ListenOptions
	Users                 []auth.User `json:"users,omitempty"`
	Network               NetworkList `json:"network,omitempty"`
	QUICCongestionControl string      `json:"quic_congestion_control,omitempty" enum:"bbr,cubic,reno"`
	InboundTLSOptionsContainer
}

type NaiveOutboundOptions struct {
	DialerOptions
	ServerOptions
	Username                 string                   `json:"username,omitempty"`
	Password                 string                   `json:"password,omitempty"`
	InsecureConcurrency      int                      `json:"insecure_concurrency,omitempty"`
	ExtraHeaders             badoption.HTTPHeader     `json:"extra_headers,omitempty"`
	ReceiveWindow            *byteformats.MemoryBytes `json:"stream_receive_window,omitempty"`
	UDPOverTCP               *UDPOverTCPOptions       `json:"udp_over_tcp,omitempty"`
	QUIC                     bool                     `json:"quic,omitempty"`
	QUICCongestionControl    string                   `json:"quic_congestion_control,omitempty" enum:"bbr,bbr2,cubic,reno"`
	QUICSessionReceiveWindow *byteformats.MemoryBytes `json:"quic_session_receive_window,omitempty"`
	OutboundTLSOptionsContainer
}
