package option

import (
	"encoding/json"

	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
)

var ErrUnknownOutboundType = E.New("unknown outbound type")

type _Outbound struct {
	Tag                string                      `json:"tag,omitempty"`
	Type               string                      `json:"type,omitempty"`
	DirectOptions      *DirectOutboundOptions      `json:"directOptions,omitempty"`
	ShadowsocksOptions *ShadowsocksOutboundOptions `json:"shadowsocksOptions,omitempty"`
}

type Outbound _Outbound

func (i *Outbound) MarshalJSON() ([]byte, error) {
	var options []byte
	var err error
	switch i.Type {
	case "direct":
		options, err = json.Marshal(i.DirectOptions)
	case "shadowsocks":
		options, err = json.Marshal(i.ShadowsocksOptions)
	default:
		return nil, E.Extend(ErrUnknownOutboundType, i.Type)
	}
	if err != nil {
		return nil, err
	}
	var content map[string]any
	err = json.Unmarshal(options, &content)
	if err != nil {
		return nil, err
	}
	content["tag"] = i.Tag
	content["type"] = i.Type
	return json.Marshal(content)
}

func (i *Outbound) UnmarshalJSON(bytes []byte) error {
	if err := json.Unmarshal(bytes, (*_Outbound)(i)); err != nil {
		return err
	}
	switch i.Type {
	case "direct":
		if i.DirectOptions != nil {
			break
		}
		if err := json.Unmarshal(bytes, &i.DirectOptions); err != nil {
			return err
		}
	case "shadowsocks":
		if i.ShadowsocksOptions != nil {
			break
		}
		if err := json.Unmarshal(bytes, &i.ShadowsocksOptions); err != nil {
			return err
		}
	default:
		return E.Extend(ErrUnknownOutboundType, i.Type)
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

type ShadowsocksOutboundOptions struct {
	DialerOptions
	ServerOptions
	Method   string `json:"method"`
	Password string `json:"password"`
}
