package option

import "github.com/sagernet/sing/common/auth"

type SocksInboundOptions struct {
	ListenOptions
	Users []auth.User `json:"users,omitempty"`
}

type HTTPMixedInboundOptions struct {
	ListenOptions
	Users          []auth.User        `json:"users,omitempty"`
	SetSystemProxy bool               `json:"set_system_proxy,omitempty"`
	TLS            *InboundTLSOptions `json:"tls,omitempty"`
}

type SocksOutboundOptions struct {
	OutboundDialerOptions
	ServerOptions
	Version  string      `json:"version,omitempty"`
	Username string      `json:"username,omitempty"`
	Password string      `json:"password,omitempty"`
	Network  NetworkList `json:"network,omitempty"`
	UoT      bool        `json:"udp_over_tcp,omitempty"`
}

type HTTPOutboundOptions struct {
	OutboundDialerOptions
	ServerOptions
	Username   string              `json:"username,omitempty"`
	Password   string              `json:"password,omitempty"`
	TLSOptions *OutboundTLSOptions `json:"tls,omitempty"`
}
