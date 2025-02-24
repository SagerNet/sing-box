package option

import "github.com/sagernet/sing/common/json/badoption"

type AnyTLSInboundOptions struct {
	ListenOptions
	InboundTLSOptionsContainer
	Users         []AnyTLSUser               `json:"users,omitempty"`
	PaddingScheme badoption.Listable[string] `json:"padding_scheme,omitempty"`
}

type AnyTLSUser struct {
	Name     string `json:"name,omitempty"`
	Password string `json:"password,omitempty"`
}

type AnyTLSOutboundOptions struct {
	DialerOptions
	ServerOptions
	OutboundTLSOptionsContainer
	Password                 string             `json:"password,omitempty"`
	IdleSessionCheckInterval badoption.Duration `json:"idle_session_check_interval,omitempty"`
	IdleSessionTimeout       badoption.Duration `json:"idle_session_timeout,omitempty"`
	MinIdleSession           int                `json:"min_idle_session,omitempty"`
}
