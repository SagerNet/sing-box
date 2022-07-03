package option

import (
	"github.com/goccy/go-json"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
)

type _Outbound struct {
	Tag                string                     `json:"tag,omitempty"`
	Type               string                     `json:"type,omitempty"`
	DirectOptions      DirectOutboundOptions      `json:"-"`
	SocksOptions       SocksOutboundOptions       `json:"-"`
	ShadowsocksOptions ShadowsocksOutboundOptions `json:"-"`
}

type Outbound _Outbound

func (h Outbound) MarshalJSON() ([]byte, error) {
	var v any
	switch h.Type {
	case "direct":
		v = h.DirectOptions
	case "socks":
		v = h.SocksOptions
	case "shadowsocks":
		v = h.ShadowsocksOptions
	default:
		return nil, E.New("unknown outbound type: ", h.Type)
	}
	return MarshallObjects((_Outbound)(h), v)
}

func (h *Outbound) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_Outbound)(h))
	if err != nil {
		return err
	}
	var v any
	switch h.Type {
	case "direct":
		v = &h.DirectOptions
	case "socks":
		v = &h.SocksOptions
	case "shadowsocks":
		v = &h.ShadowsocksOptions
	default:
		return nil
	}
	err = UnmarshallExcluded(bytes, (*_Outbound)(h), v)
	if err != nil {
		return E.Cause(err, "outbound options")
	}
	return nil
}

type DialerOptions struct {
	Detour         string `json:"detour,omitempty"`
	BindInterface  string `json:"bind_interface,omitempty"`
	RoutingMark    int    `json:"routing_mark,omitempty"`
	ReuseAddr      bool   `json:"reuse_addr,omitempty"`
	ConnectTimeout int    `json:"connect_timeout,omitempty"`
	TCPFastOpen    bool   `json:"tcp_fast_open,omitempty"`
}

type DirectOutboundOptions struct {
	DialerOptions
	OverrideAddress string `json:"override_address,omitempty"`
	OverridePort    uint16 `json:"override_port,omitempty"`
}

type ServerOptions struct {
	Server     string `json:"server"`
	ServerPort uint16 `json:"server_port"`
}

func (o ServerOptions) Build() M.Socksaddr {
	return M.ParseSocksaddrHostPort(o.Server, o.ServerPort)
}

type SocksOutboundOptions struct {
	DialerOptions
	ServerOptions
	Version  string `json:"version,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

type ShadowsocksOutboundOptions struct {
	DialerOptions
	ServerOptions
	Method   string `json:"method"`
	Password string `json:"password"`
}
