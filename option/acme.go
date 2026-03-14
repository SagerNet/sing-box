package option

import "github.com/sagernet/sing/common/json/badoption"

type ACMEServiceOptions struct {
	Domain                  badoption.Listable[string]  `json:"domain,omitempty"`
	DataDirectory           string                      `json:"data_directory,omitempty"`
	DefaultServerName       string                      `json:"default_server_name,omitempty"`
	Email                   string                      `json:"email,omitempty"`
	Provider                string                      `json:"provider,omitempty"`
	DisableHTTPChallenge    bool                        `json:"disable_http_challenge,omitempty"`
	DisableTLSALPNChallenge bool                        `json:"disable_tls_alpn_challenge,omitempty"`
	AlternativeHTTPPort     uint16                      `json:"alternative_http_port,omitempty"`
	AlternativeTLSPort      uint16                      `json:"alternative_tls_port,omitempty"`
	ExternalAccount         *ACMEExternalAccountOptions `json:"external_account,omitempty"`
	DNS01Challenge          *ACMEDNS01ChallengeOptions  `json:"dns01_challenge,omitempty"`
}
