package option

import "github.com/sagernet/sing/common/json/badoption"

type APIServiceOptions struct {
	ListenOptions
	Secret                           string                     `json:"secret,omitempty"`
	AccessControlAllowOrigin         badoption.Listable[string] `json:"access_control_allow_origin,omitempty"`
	AccessControlAllowPrivateNetwork bool                       `json:"access_control_allow_private_network,omitempty"`
	InboundTLSOptionsContainer
}
