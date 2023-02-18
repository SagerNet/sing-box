package ssmapi

import (
	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

var _ Node = (*ShadowsocksNode)(nil)

type ShadowsocksNode struct {
	node    option.SSMShadowsocksNode
	inbound adapter.ManagedShadowsocksServer
}

type ShadowsocksNodeObject struct {
	ID        string   `json:"id,omitempty"`
	Name      string   `json:"name,omitempty"`
	Endpoint  string   `json:"endpoint,omitempty"`
	Method    string   `json:"method,omitempty"`
	Passwords []string `json:"iPSKs,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

func (n *ShadowsocksNode) Protocol() string {
	return C.TypeShadowsocks
}

func (n *ShadowsocksNode) ID() string {
	return n.node.ID
}

func (n *ShadowsocksNode) Shadowsocks() ShadowsocksNodeObject {
	return ShadowsocksNodeObject{
		ID:        n.node.ID,
		Name:      n.node.Name,
		Endpoint:  n.node.Address,
		Method:    n.inbound.Method(),
		Passwords: []string{n.inbound.Password()},
		Tags:      n.node.Tags,
	}
}

func (n *ShadowsocksNode) Object() any {
	return n.Shadowsocks()
}

func (n *ShadowsocksNode) Tag() string {
	return n.inbound.Tag()
}

func (n *ShadowsocksNode) UpdateUsers(users []string, uPSKs []string) error {
	return n.inbound.UpdateUsers(users, uPSKs)
}
