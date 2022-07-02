package option

import (
	"encoding/json"

	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
)

type _Outbound struct {
	Tag                string                     `json:"tag,omitempty"`
	Type               string                     `json:"type,omitempty"`
	DirectOptions      DirectOutboundOptions      `json:"-"`
	ShadowsocksOptions ShadowsocksOutboundOptions `json:"-"`
}

type Outbound _Outbound

func (i *Outbound) MarshalJSON() ([]byte, error) {
	var v any
	switch i.Type {
	case "direct":
		v = i.DirectOptions
	case "shadowsocks":
		v = i.ShadowsocksOptions
	default:
		return nil, E.New("unknown outbound type: ", i.Type)
	}
	return MarshallObjects(i, v)
}

func (i *Outbound) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_Outbound)(i))
	if err != nil {
		return err
	}
	var v any
	switch i.Type {
	case "direct":
		v = &i.DirectOptions
	case "shadowsocks":
		v = &i.ShadowsocksOptions
	default:
		return nil
	}
	return json.Unmarshal(bytes, v)
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

type ShadowsocksOutboundOptions struct {
	DialerOptions
	ServerOptions
	Method   string `json:"method"`
	Password string `json:"password"`
}
