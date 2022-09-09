package option

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

type OutboundTLSOptions struct {
	Enabled         bool                `json:"enabled,omitempty"`
	DisableSNI      bool                `json:"disable_sni,omitempty"`
	ServerName      string              `json:"server_name,omitempty"`
	Insecure        bool                `json:"insecure,omitempty"`
	ALPN            Listable[string]    `json:"alpn,omitempty"`
	MinVersion      string              `json:"min_version,omitempty"`
	MaxVersion      string              `json:"max_version,omitempty"`
	CipherSuites    Listable[string]    `json:"cipher_suites,omitempty"`
	Certificate     string              `json:"certificate,omitempty"`
	CertificatePath string              `json:"certificate_path,omitempty"`
	ECH             *OutboundECHOptions `json:"ech,omitempty"`
}

type OutboundECHOptions struct {
	Enabled                     bool   `json:"enabled,omitempty"`
	PQSignatureSchemesEnabled   bool   `json:"pq_signature_schemes_enabled,omitempty"`
	DynamicRecordSizingDisabled bool   `json:"dynamic_record_sizing_disabled,omitempty"`
	Config                      string `json:"config,omitempty"`
}
