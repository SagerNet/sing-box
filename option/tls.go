package option

import (
	"crypto/tls"

	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

type InboundTLSOptions struct {
	Enabled         bool     `json:"enabled,omitempty"`
	ServerName      string   `json:"server_name,omitempty"`
	ALPN            []string `json:"alpn,omitempty"`
	MinVersion      string   `json:"min_version,omitempty"`
	MaxVersion      string   `json:"max_version,omitempty"`
	CipherSuites    []string `json:"cipher_suites,omitempty"`
	Certificate     string   `json:"certificate,omitempty"`
	CertificatePath string   `json:"certificate_path,omitempty"`
	Key             string   `json:"key,omitempty"`
	KeyPath         string   `json:"key_path,omitempty"`
}

func (o InboundTLSOptions) Equals(other InboundTLSOptions) bool {
	return o.Enabled == other.Enabled &&
		o.ServerName == other.ServerName &&
		common.ComparableSliceEquals(o.ALPN, other.ALPN) &&
		o.MinVersion == other.MinVersion &&
		o.MaxVersion == other.MaxVersion &&
		common.ComparableSliceEquals(o.CipherSuites, other.CipherSuites) &&
		o.Certificate == other.Certificate &&
		o.CertificatePath == other.CertificatePath &&
		o.Key == other.Key &&
		o.KeyPath == other.KeyPath
}

type OutboundTLSOptions struct {
	Enabled           bool     `json:"enabled,omitempty"`
	DisableSNI        bool     `json:"disable_sni,omitempty"`
	ServerName        string   `json:"server_name,omitempty"`
	Insecure          bool     `json:"insecure,omitempty"`
	ALPN              []string `json:"alpn,omitempty"`
	MinVersion        string   `json:"min_version,omitempty"`
	MaxVersion        string   `json:"max_version,omitempty"`
	CipherSuites      []string `json:"cipher_suites,omitempty"`
	DisableSystemRoot bool     `json:"disable_system_root,omitempty"`
	Certificate       string   `json:"certificate,omitempty"`
	CertificatePath   string   `json:"certificate_path,omitempty"`
}

func (o OutboundTLSOptions) Equals(other OutboundTLSOptions) bool {
	return o.Enabled == other.Enabled &&
		o.DisableSNI == other.DisableSNI &&
		o.ServerName == other.ServerName &&
		o.Insecure == other.Insecure &&
		common.ComparableSliceEquals(o.ALPN, other.ALPN) &&
		o.MinVersion == other.MinVersion &&
		o.MaxVersion == other.MaxVersion &&
		common.ComparableSliceEquals(o.CipherSuites, other.CipherSuites) &&
		o.DisableSystemRoot == other.DisableSystemRoot &&
		o.Certificate == other.Certificate &&
		o.CertificatePath == other.CertificatePath
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
