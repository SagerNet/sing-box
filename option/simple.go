package option

import "github.com/sagernet/sing/common/auth"

type SocksInboundOptions struct {
	ListenOptions
	Users []auth.User `json:"users,omitempty"`
}

type HTTPMixedInboundOptions struct {
	ListenOptions
	Users          []auth.User `json:"users,omitempty"`
	SetSystemProxy bool        `json:"set_system_proxy,omitempty"`
	InboundTLSOptionsContainer
}

type SocksOutboundOptions struct {
	DialerOptions
	ServerOptions
	Version    string             `json:"version,omitempty"`
	Username   string             `json:"username,omitempty"`
	Password   string             `json:"password,omitempty"`
	Network    NetworkList        `json:"network,omitempty"`
	UDPOverTCP *UDPOverTCPOptions `json:"udp_over_tcp,omitempty"`
}

type HTTPOutboundOptions struct {
	DialerOptions
	ServerOptions
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	OutboundTLSOptionsContainer
	Path    string     `json:"path,omitempty"`
	Headers HTTPHeader `json:"headers,omitempty"`
}
