package option

import "github.com/sagernet/sing/common/json/badoption"

type InboundTLSOptions struct {
	Enabled         bool                       `json:"enabled,omitempty"`
	ServerName      string                     `json:"server_name,omitempty"`
	Insecure        bool                       `json:"insecure,omitempty"`
	ALPN            badoption.Listable[string] `json:"alpn,omitempty"`
	MinVersion      string                     `json:"min_version,omitempty"`
	MaxVersion      string                     `json:"max_version,omitempty"`
	CipherSuites    badoption.Listable[string] `json:"cipher_suites,omitempty"`
	Certificate     badoption.Listable[string] `json:"certificate,omitempty"`
	CertificatePath string                     `json:"certificate_path,omitempty"`
	Key             badoption.Listable[string] `json:"key,omitempty"`
	KeyPath         string                     `json:"key_path,omitempty"`
	ACME            *InboundACMEOptions        `json:"acme,omitempty"`
	ECH             *InboundECHOptions         `json:"ech,omitempty"`
	Reality         *InboundRealityOptions     `json:"reality,omitempty"`
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
	Enabled         bool                       `json:"enabled,omitempty"`
	DisableSNI      bool                       `json:"disable_sni,omitempty"`
	ServerName      string                     `json:"server_name,omitempty"`
	Insecure        bool                       `json:"insecure,omitempty"`
	ALPN            badoption.Listable[string] `json:"alpn,omitempty"`
	MinVersion      string                     `json:"min_version,omitempty"`
	MaxVersion      string                     `json:"max_version,omitempty"`
	CipherSuites    badoption.Listable[string] `json:"cipher_suites,omitempty"`
	Certificate     badoption.Listable[string] `json:"certificate,omitempty"`
	CertificatePath string                     `json:"certificate_path,omitempty"`
	ECH             *OutboundECHOptions        `json:"ech,omitempty"`
	UTLS            *OutboundUTLSOptions       `json:"utls,omitempty"`
	Reality         *OutboundRealityOptions    `json:"reality,omitempty"`
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
	Enabled                     bool                       `json:"enabled,omitempty"`
	PQSignatureSchemesEnabled   bool                       `json:"pq_signature_schemes_enabled,omitempty"`
	DynamicRecordSizingDisabled bool                       `json:"dynamic_record_sizing_disabled,omitempty"`
	Key                         badoption.Listable[string] `json:"key,omitempty"`
	KeyPath                     string                     `json:"key_path,omitempty"`
}

type OutboundECHOptions struct {
	Enabled                     bool                       `json:"enabled,omitempty"`
	PQSignatureSchemesEnabled   bool                       `json:"pq_signature_schemes_enabled,omitempty"`
	DynamicRecordSizingDisabled bool                       `json:"dynamic_record_sizing_disabled,omitempty"`
	Config                      badoption.Listable[string] `json:"config,omitempty"`
	ConfigPath                  string                     `json:"config_path,omitempty"`
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
