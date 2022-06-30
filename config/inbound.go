package config

import (
	"encoding/json"

	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
)

var ErrUnknownInboundType = E.New("unknown inbound type")

type _Inbound struct {
	Tag                string                     `json:"tag,omitempty"`
	Type               string                     `json:"type,omitempty"`
	DirectOptions      *DirectInboundOptions      `json:"directOptions,omitempty"`
	SocksOptions       *SimpleInboundOptions      `json:"socksOptions,omitempty"`
	HTTPOptions        *SimpleInboundOptions      `json:"httpOptions,omitempty"`
	MixedOptions       *SimpleInboundOptions      `json:"mixedOptions,omitempty"`
	ShadowsocksOptions *ShadowsocksInboundOptions `json:"shadowsocksOptions,omitempty"`
}

type Inbound _Inbound

func (i *Inbound) MarshalJSON() ([]byte, error) {
	var options []byte
	var err error
	switch i.Type {
	case "direct":
		options, err = json.Marshal(i.DirectOptions)
	case "socks":
		options, err = json.Marshal(i.SocksOptions)
	case "http":
		options, err = json.Marshal(i.HTTPOptions)
	case "mixed":
		options, err = json.Marshal(i.MixedOptions)
	case "shadowsocks":
		options, err = json.Marshal(i.ShadowsocksOptions)
	default:
		return nil, E.Extend(ErrUnknownInboundType, i.Type)
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

func (i *Inbound) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_Inbound)(i))
	if err != nil {
		return err
	}
	switch i.Type {
	case "direct":
		if i.DirectOptions != nil {
			break
		}
		err = json.Unmarshal(bytes, &i.DirectOptions)
	case "socks":
		if i.SocksOptions != nil {
			break
		}
		err = json.Unmarshal(bytes, &i.SocksOptions)
	case "http":
		if i.HTTPOptions != nil {
			break
		}
		err = json.Unmarshal(bytes, &i.HTTPOptions)
	case "mixed":
		if i.MixedOptions != nil {
			break
		}
		err = json.Unmarshal(bytes, &i.MixedOptions)
	case "shadowsocks":
		if i.ShadowsocksOptions != nil {
			break
		}
		err = json.Unmarshal(bytes, &i.ShadowsocksOptions)
	default:
		return E.Extend(ErrUnknownInboundType, i.Type)
	}
	return err
}

type ListenOptions struct {
	Listen      ListenAddress `json:"listen"`
	Port        uint16        `json:"listen_port"`
	TCPFastOpen bool          `json:"tcpFastOpen,omitempty"`
	UDPTimeout  int64         `json:"udpTimeout,omitempty"`
}

type SimpleInboundOptions struct {
	ListenOptions
	Users []auth.User `json:"users,omitempty"`
}

type DirectInboundOptions struct {
	ListenOptions
	Network         NetworkList `json:"network,omitempty"`
	OverrideAddress string      `json:"overrideAddress,omitempty"`
	OverridePort    uint16      `json:"overridePort,omitempty"`
}

type ShadowsocksInboundOptions struct {
	ListenOptions
	Network  NetworkList `json:"network,omitempty"`
	Method   string      `json:"method"`
	Password string      `json:"password"`
}
