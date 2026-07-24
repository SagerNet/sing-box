package option

type ShadowsocksInboundOptions struct {
	ListenOptions
	Network      NetworkList              `json:"network,omitempty"`
	Method       string                   `json:"method" enum:"none,aes-128-gcm,aes-192-gcm,aes-256-gcm,chacha20-ietf-poly1305,xchacha20-ietf-poly1305,2022-blake3-aes-128-gcm,2022-blake3-aes-256-gcm,2022-blake3-chacha20-poly1305"`
	Password     string                   `json:"password,omitempty"`
	Users        []ShadowsocksUser        `json:"users,omitempty"`
	Destinations []ShadowsocksDestination `json:"destinations,omitempty"`
	Multiplex    *InboundMultiplexOptions `json:"multiplex,omitempty"`
	Managed      bool                     `json:"managed,omitempty"`
}

type ShadowsocksUser struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type ShadowsocksDestination struct {
	Name     string `json:"name"`
	Password string `json:"password"`
	ServerOptions
}

type ShadowsocksOutboundOptions struct {
	DialerOptions
	ServerOptions
	Method        string                    `json:"method" enum:"none,aes-128-gcm,aes-192-gcm,aes-256-gcm,chacha20-ietf-poly1305,xchacha20-ietf-poly1305,2022-blake3-aes-128-gcm,2022-blake3-aes-256-gcm,2022-blake3-chacha20-poly1305,aes-128-ctr,aes-192-ctr,aes-256-ctr,aes-128-cfb,aes-192-cfb,aes-256-cfb,rc4-md5,chacha20-ietf,xchacha20"`
	Password      string                    `json:"password"`
	Plugin        string                    `json:"plugin,omitempty"`
	PluginOptions string                    `json:"plugin_opts,omitempty"`
	Network       NetworkList               `json:"network,omitempty"`
	UDPOverTCP    *UDPOverTCPOptions        `json:"udp_over_tcp,omitempty"`
	Multiplex     *OutboundMultiplexOptions `json:"multiplex,omitempty"`
}
