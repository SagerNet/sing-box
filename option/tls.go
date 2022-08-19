package option

import (
	"crypto/tls"

	E "github.com/sagernet/sing/common/exceptions"
)

type InboundTLSOptions struct {
	Enabled         bool                `json:"enabled,omitempty"`
	ServerName      string              `json:"server_name,omitempty"`
	ALPN            Listable[string]    `json:"alpn,omitempty"`
	MinVersion      string              `json:"min_version,omitempty"`
	MaxVersion      string              `json:"max_version,omitempty"`
	CipherSuites    Listable[string]    `json:"cipher_suites,omitempty"`
	Certificate     string              `json:"certificate,omitempty"`
	CertificatePath string              `json:"certificate_path,omitempty"`
	Key             string              `json:"key,omitempty"`
	KeyPath         string              `json:"key_path,omitempty"`
	ACME            *InboundACMEOptions `json:"acme,omitempty"`
}

type InboundACMEOptions struct {
	Domain                  Listable[string] `json:"domain,omitempty"`
	DataDirectory           string           `json:"data_directory,omitempty"`
	DefaultServerName       string           `json:"default_server_name,omitempty"`
	Email                   string           `json:"email,omitempty"`
	Provider                string           `json:"provider,omitempty"`
	DisableHTTPChallenge    bool             `json:"disable_http_challenge,omitempty"`
	DisableTLSALPNChallenge bool             `json:"disable_tls_alpn_challenge,omitempty"`
	AlternativeHTTPPort     uint16           `json:"alternative_http_port,omitempty"`
	AlternativeTLSPort      uint16           `json:"alternative_tls_port,omitempty"`
}

type OutboundTLSOptions struct {
	Enabled         bool             `json:"enabled,omitempty"`
	DisableSNI      bool             `json:"disable_sni,omitempty"`
	ServerName      string           `json:"server_name,omitempty"`
	Insecure        bool             `json:"insecure,omitempty"`
	ALPN            Listable[string] `json:"alpn,omitempty"`
	MinVersion      string           `json:"min_version,omitempty"`
	MaxVersion      string           `json:"max_version,omitempty"`
	CipherSuites    Listable[string] `json:"cipher_suites,omitempty"`
	Certificate     string           `json:"certificate,omitempty"`
	CertificatePath string           `json:"certificate_path,omitempty"`
}

func ParseTLSVersion(version string) (uint16, error) {
	switch version {
	case "1.0":
		return tls.VersionTLS10, nil
	case "1.1":
		return tls.VersionTLS11, nil
	case "1.2":
		return tls.VersionTLS12, nil
	case "1.3":
		return tls.VersionTLS13, nil
	default:
		return 0, E.New("unknown tls version:", version)
	}
}
