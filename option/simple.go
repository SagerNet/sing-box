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
	DialerOptions
	ServerOptions
	Version           string             `json:"version,omitempty"`
	Username          string             `json:"username,omitempty"`
	Password          string             `json:"password,omitempty"`
	Network           NetworkList        `json:"network,omitempty"`
	UDPOverTCPOptions *UDPOverTCPOptions `json:"udp_over_tcp,omitempty"`
}

type HTTPOutboundOptions struct {
	DialerOptions
	ServerOptions
	Username string                      `json:"username,omitempty"`
	Password string                      `json:"password,omitempty"`
	TLS      *OutboundTLSOptions         `json:"tls,omitempty"`
	Path     string                      `json:"path,omitempty"`
	Headers  map[string]Listable[string] `json:"headers,omitempty"`
}
