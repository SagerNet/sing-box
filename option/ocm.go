package option

import (
	"github.com/sagernet/sing/common/json/badoption"
)

type OCMServiceOptions struct {
	ListenOptions
	InboundTLSOptionsContainer
	CredentialPath string               `json:"credential_path,omitempty"`
	Users          []OCMUser            `json:"users,omitempty"`
	Headers        badoption.HTTPHeader `json:"headers,omitempty"`
	Detour         string               `json:"detour,omitempty"`
	UsagesPath     string               `json:"usages_path,omitempty"`
}

type OCMUser struct {
	Name  string `json:"name,omitempty"`
	Token string `json:"token,omitempty"`
}
