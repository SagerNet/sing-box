package option

import (
	"crypto/tls"
	"encoding/json"
	"strings"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json/badoption"
)

type InboundTLSOptions struct {
	Enabled                          bool                                `json:"enabled,omitempty"`
	ServerName                       string                              `json:"server_name,omitempty"`
	Insecure                         bool                                `json:"insecure,omitempty"`
	ALPN                             badoption.Listable[string]          `json:"alpn,omitempty"`
	MinVersion                       string                              `json:"min_version,omitempty"`
	MaxVersion                       string                              `json:"max_version,omitempty"`
	CipherSuites                     badoption.Listable[string]          `json:"cipher_suites,omitempty"`
	CurvePreferences                 badoption.Listable[CurvePreference] `json:"curve_preferences,omitempty"`
	Certificate                      badoption.Listable[string]          `json:"certificate,omitempty"`
	CertificatePath                  string                              `json:"certificate_path,omitempty"`
	ClientAuthentication             ClientAuthType                      `json:"client_authentication,omitempty"`
	ClientCertificate                badoption.Listable[string]          `json:"client_certificate,omitempty"`
	ClientCertificatePath            badoption.Listable[string]          `json:"client_certificate_path,omitempty"`
	ClientCertificatePublicKeySHA256 badoption.Listable[[]byte]          `json:"client_certificate_public_key_sha256,omitempty"`
	Key                              badoption.Listable[string]          `json:"key,omitempty"`
	KeyPath                          string                              `json:"key_path,omitempty"`
	KernelTx                         bool                                `json:"kernel_tx,omitempty"`
	KernelRx                         bool                                `json:"kernel_rx,omitempty"`
	ACME                             *InboundACMEOptions                 `json:"acme,omitempty"`
	ECH                              *InboundECHOptions                  `json:"ech,omitempty"`
	Reality                          *InboundRealityOptions              `json:"reality,omitempty"`
}

type ClientAuthType tls.ClientAuthType

func (t ClientAuthType) MarshalJSON() ([]byte, error) {
	var stringValue string
	switch t {
	case ClientAuthType(tls.NoClientCert):
		stringValue = "no"
	case ClientAuthType(tls.RequestClientCert):
		stringValue = "request"
	case ClientAuthType(tls.RequireAnyClientCert):
		stringValue = "require-any"
	case ClientAuthType(tls.VerifyClientCertIfGiven):
		stringValue = "verify-if-given"
	case ClientAuthType(tls.RequireAndVerifyClientCert):
		stringValue = "require-and-verify"
	default:
		return nil, E.New("unknown client authentication type: ", int(t))
	}
	return json.Marshal(stringValue)
}

func (t *ClientAuthType) UnmarshalJSON(data []byte) error {
	var stringValue string
	err := json.Unmarshal(data, &stringValue)
	if err != nil {
		return err
	}
	switch stringValue {
	case "no":
		*t = ClientAuthType(tls.NoClientCert)
	case "request":
		*t = ClientAuthType(tls.RequestClientCert)
	case "require-any":
		*t = ClientAuthType(tls.RequireAnyClientCert)
	case "verify-if-given":
		*t = ClientAuthType(tls.VerifyClientCertIfGiven)
	case "require-and-verify":
		*t = ClientAuthType(tls.RequireAndVerifyClientCert)
	default:
		return E.New("unknown client authentication type: ", stringValue)
	}
	return nil
}

type InboundTLSOptionsContainer struct {
	TLS *InboundTLSOptions `json:"tls,omitempty"`
}

type InboundTLSOptionsWrapper interface {
	TakeInboundTLSOptions() *InboundTLSOptions
	ReplaceInboundTLSOptions(options *InboundTLSOptions)
}

func (o *InboundTLSOptionsContainer) TakeInboundTLSOptions() *InboundTLSOptions {
	return o.TLS
}

func (o *InboundTLSOptionsContainer) ReplaceInboundTLSOptions(options *InboundTLSOptions) {
	o.TLS = options
}

type OutboundTLSOptions struct {
	Enabled                    bool                                `json:"enabled,omitempty"`
	DisableSNI                 bool                                `json:"disable_sni,omitempty"`
	ServerName                 string                              `json:"server_name,omitempty"`
	Insecure                   bool                                `json:"insecure,omitempty"`
	ALPN                       badoption.Listable[string]          `json:"alpn,omitempty"`
	MinVersion                 string                              `json:"min_version,omitempty"`
	MaxVersion                 string                              `json:"max_version,omitempty"`
	CipherSuites               badoption.Listable[string]          `json:"cipher_suites,omitempty"`
	CurvePreferences           badoption.Listable[CurvePreference] `json:"curve_preferences,omitempty"`
	Certificate                badoption.Listable[string]          `json:"certificate,omitempty"`
	CertificatePath            string                              `json:"certificate_path,omitempty"`
	CertificatePublicKeySHA256 badoption.Listable[[]byte]          `json:"certificate_public_key_sha256,omitempty"`
	ClientCertificate          badoption.Listable[string]          `json:"client_certificate,omitempty"`
	ClientCertificatePath      string                              `json:"client_certificate_path,omitempty"`
	ClientKey                  badoption.Listable[string]          `json:"client_key,omitempty"`
	ClientKeyPath              string                              `json:"client_key_path,omitempty"`
	Fragment                   bool                                `json:"fragment,omitempty"`
	FragmentFallbackDelay      badoption.Duration                  `json:"fragment_fallback_delay,omitempty"`
	RecordFragment             bool                                `json:"record_fragment,omitempty"`
	KernelTx                   bool                                `json:"kernel_tx,omitempty"`
	KernelRx                   bool                                `json:"kernel_rx,omitempty"`
	ECH                        *OutboundECHOptions                 `json:"ech,omitempty"`
	UTLS                       *OutboundUTLSOptions                `json:"utls,omitempty"`
	Reality                    *OutboundRealityOptions             `json:"reality,omitempty"`
}

type OutboundTLSOptionsContainer struct {
	TLS *OutboundTLSOptions `json:"tls,omitempty"`
}

type OutboundTLSOptionsWrapper interface {
	TakeOutboundTLSOptions() *OutboundTLSOptions
	ReplaceOutboundTLSOptions(options *OutboundTLSOptions)
}

func (o *OutboundTLSOptionsContainer) TakeOutboundTLSOptions() *OutboundTLSOptions {
	return o.TLS
}

func (o *OutboundTLSOptionsContainer) ReplaceOutboundTLSOptions(options *OutboundTLSOptions) {
	o.TLS = options
}

type CurvePreference tls.CurveID

const (
	CurveP256      = 23
	CurveP384      = 24
	CurveP521      = 25
	X25519         = 29
	X25519MLKEM768 = 4588
)

func (c CurvePreference) MarshalJSON() ([]byte, error) {
	var stringValue string
	switch c {
	case CurvePreference(CurveP256):
		stringValue = "P256"
	case CurvePreference(CurveP384):
		stringValue = "P384"
	case CurvePreference(CurveP521):
		stringValue = "P521"
	case CurvePreference(X25519):
		stringValue = "X25519"
	case CurvePreference(X25519MLKEM768):
		stringValue = "X25519MLKEM768"
	default:
		return nil, E.New("unknown curve id: ", int(c))
	}
	return json.Marshal(stringValue)
}

func (c *CurvePreference) UnmarshalJSON(data []byte) error {
	var stringValue string
	err := json.Unmarshal(data, &stringValue)
	if err != nil {
		return err
	}
	switch strings.ToUpper(stringValue) {
	case "P256":
		*c = CurvePreference(CurveP256)
	case "P384":
		*c = CurvePreference(CurveP384)
	case "P521":
		*c = CurvePreference(CurveP521)
	case "X25519":
		*c = CurvePreference(X25519)
	case "X25519MLKEM768":
		*c = CurvePreference(X25519MLKEM768)
	default:
		return E.New("unknown curve name: ", stringValue)
	}
	return nil
}

type InboundRealityOptions struct {
	Enabled           bool                           `json:"enabled,omitempty"`
	Handshake         InboundRealityHandshakeOptions `json:"handshake,omitempty"`
	PrivateKey        string                         `json:"private_key,omitempty"`
	ShortID           badoption.Listable[string]     `json:"short_id,omitempty"`
	MaxTimeDifference badoption.Duration             `json:"max_time_difference,omitempty"`
}

type InboundRealityHandshakeOptions struct {
	ServerOptions
	DialerOptions
}

type InboundECHOptions struct {
	Enabled bool                       `json:"enabled,omitempty"`
	Key     badoption.Listable[string] `json:"key,omitempty"`
	KeyPath string                     `json:"key_path,omitempty"`

	// Deprecated: not supported by stdlib
	PQSignatureSchemesEnabled bool `json:"pq_signature_schemes_enabled,omitempty"`
	// Deprecated: added by fault
	DynamicRecordSizingDisabled bool `json:"dynamic_record_sizing_disabled,omitempty"`
}

type OutboundECHOptions struct {
	Enabled    bool                       `json:"enabled,omitempty"`
	Config     badoption.Listable[string] `json:"config,omitempty"`
	ConfigPath string                     `json:"config_path,omitempty"`

	// Deprecated: not supported by stdlib
	PQSignatureSchemesEnabled bool `json:"pq_signature_schemes_enabled,omitempty"`
	// Deprecated: added by fault
	DynamicRecordSizingDisabled bool `json:"dynamic_record_sizing_disabled,omitempty"`
}

type OutboundUTLSOptions struct {
	Enabled     bool   `json:"enabled,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

type OutboundRealityOptions struct {
	Enabled   bool   `json:"enabled,omitempty"`
	PublicKey string `json:"public_key,omitempty"`
	ShortID   string `json:"short_id,omitempty"`
}
