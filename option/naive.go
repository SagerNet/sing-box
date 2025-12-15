package option

import (
	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/json/badoption"
)

type NaiveInboundOptions struct {
	ListenOptions
	Users   []auth.User `json:"users,omitempty"`
	Network NetworkList `json:"network,omitempty"`
	InboundTLSOptionsContainer
}

type NaiveOutboundOptions struct {
	DialerOptions
	ServerOptions
	Username            string               `json:"username,omitempty"`
	Password            string               `json:"password,omitempty"`
	InsecureConcurrency int                  `json:"insecure_concurrency,omitempty"`
	ExtraHeaders        badoption.HTTPHeader `json:"extra_headers,omitempty"`
	UDPOverTCP          *UDPOverTCPOptions   `json:"udp_over_tcp,omitempty"`
	OutboundTLSOptionsContainer
}
