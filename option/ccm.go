package option

import (
	"github.com/sagernet/sing/common/json/badoption"
)

type CCMServiceOptions struct {
	ListenOptions
	InboundTLSOptionsContainer
	CredentialPath string               `json:"credential_path,omitempty"`
	Users          []CCMUser            `json:"users,omitempty"`
	Headers        badoption.HTTPHeader `json:"headers,omitempty"`
	Detour         string               `json:"detour,omitempty"`
}

type CCMUser struct {
	Name  string `json:"name,omitempty"`
	Token string `json:"token,omitempty"`
}
