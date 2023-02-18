package option

import (
	"github.com/sagernet/sing-box/common/json"
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
)

type SSMAPIOptions struct {
	Listen       string    `json:"listen,omitempty"`
	ListenPrefix string    `json:"listen_prefix,omitempty"`
	Nodes        []SSMNode `json:"nodes,omitempty"`
}

type _SSMNode struct {
	Type               string             `json:"type,omitempty"`
	ShadowsocksOptions SSMShadowsocksNode `json:"-"`
}

type SSMNode _SSMNode

func (h SSMNode) MarshalJSON() ([]byte, error) {
	var v any
	switch h.Type {
	case C.TypeShadowsocks:
		v = h.ShadowsocksOptions
	default:
		return nil, E.New("unknown ssm node type: ", h.Type)
	}
	return MarshallObjects((_SSMNode)(h), v)
}

func (h *SSMNode) UnmarshalJSON(data []byte) error {
	err := json.Unmarshal(data, (*_SSMNode)(h))
	if err != nil {
		return err
	}
	var v any
	switch h.Type {
	case C.TypeShadowsocks:
		v = &h.ShadowsocksOptions
	default:
		return E.New("unknown ssm node type: ", h.Type)
	}
	return UnmarshallExcluded(data, (*_SSMNode)(h), v)
}

type SSMShadowsocksNode struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Address string   `json:"address"`
	Tags    []string `json:"tags"`
	Inbound string   `json:"inbound"`
}
