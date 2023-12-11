package option

import "github.com/sagernet/sing/common/auth"

type NaiveInboundOptions struct {
	ListenOptions
	Users   []auth.User `json:"users,omitempty"`
	Network NetworkList `json:"network,omitempty"`
	InboundTLSOptionsContainer
}
