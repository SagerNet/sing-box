package option

import "github.com/sagernet/sing/common/json/badoption"

type AnyTLS2InboundOptions struct {
	ListenOptions
	InboundTLSOptionsContainer
	Users         []AnyTLSUser               `json:"users,omitempty"`
	PaddingScheme badoption.Listable[string] `json:"padding_scheme,omitempty"`
	Transport     *V2RayTransportOptions     `json:"transport,omitempty"`
}

type AnyTLS2OutboundOptions struct {
	DialerOptions
	ServerOptions
	OutboundTLSOptionsContainer
	Password                 string                 `json:"password,omitempty"`
	IdleSessionCheckInterval badoption.Duration     `json:"idle_session_check_interval,omitempty"`
	IdleSessionTimeout       badoption.Duration     `json:"idle_session_timeout,omitempty"`
	MinIdleSession           int                    `json:"min_idle_session,omitempty"`
	Transport                *V2RayTransportOptions `json:"transport,omitempty"`
}
