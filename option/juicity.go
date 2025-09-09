package option

import "github.com/sagernet/sing/common/json/badoption"

type JuicityInboundOptions struct {
	ListenOptions
	Users       []JuicityUser      `json:"users,omitempty"`
	AuthTimeout badoption.Duration `json:"auth_timeout,omitempty"`
	InboundTLSOptionsContainer

	SpeedTest string `json:"speed_test,omitempty"`
}

type JuicityUser struct {
	Name     string `json:"name,omitempty"`
	UUID     string `json:"uuid,omitempty"`
	Password string `json:"password,omitempty"`
}

type JuicityOutboundOptions struct {
	DialerOptions
	ServerOptions
	UUID     string `json:"uuid,omitempty"`
	Password string `json:"password,omitempty"`
	OutboundTLSOptionsContainer
}
