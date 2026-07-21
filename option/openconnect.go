package option

import "github.com/sagernet/sing/common/json/badoption"

type OpenConnectEndpointOptions struct {
	DialerOptions
	System                         bool                          `json:"system,omitempty"`
	Name                           string                        `json:"name,omitempty"`
	UDPTimeout                     badoption.Duration            `json:"udp_timeout,omitempty"`
	UDPMapping                     UDPNATBehavior                `json:"udp_mapping,omitempty"`
	UDPFiltering                   UDPNATBehavior                `json:"udp_filtering,omitempty"`
	UDPNATMax                      uint32                        `json:"udp_nat_max,omitempty"`
	Server                         string                        `json:"server"`
	Flavor                         string                        `json:"flavor,omitempty"`
	Username                       string                        `json:"username,omitempty"`
	Password                       string                        `json:"password,omitempty"`
	AuthGroup                      string                        `json:"auth_group,omitempty"`
	Cookie                         string                        `json:"cookie,omitempty"`
	Token                          *OpenConnectTokenOptions      `json:"token,omitempty"`
	ReportedOS                     string                        `json:"reported_os,omitempty"`
	UserAgent                      string                        `json:"user_agent,omitempty"`
	Version                        string                        `json:"version,omitempty"`
	LocalHostname                  string                        `json:"local_hostname,omitempty"`
	Mobile                         *OpenConnectMobileOptions     `json:"mobile,omitempty"`
	CSD                            *OpenConnectCSDOptions        `json:"csd,omitempty"`
	HIP                            *OpenConnectHIPOptions        `json:"hip,omitempty"`
	TNCC                           *OpenConnectTNCCOptions       `json:"tncc,omitempty"`
	NoUDP                          bool                          `json:"no_udp,omitempty"`
	DTLSLocalPort                  uint16                        `json:"dtls_local_port,omitempty"`
	CompressionDisabled            bool                          `json:"compression_disabled,omitempty"`
	CompressionMode                string                        `json:"compression_mode,omitempty"`
	IPv6Disabled                   bool                          `json:"ipv6_disabled,omitempty"`
	HTTPKeepAliveDisabled          bool                          `json:"http_keepalive_disabled,omitempty"`
	XMLPostDisabled                bool                          `json:"xml_post_disabled,omitempty"`
	ExternalAuthDisabled           bool                          `json:"external_auth_disabled,omitempty"`
	PasswordAuthenticationDisabled bool                          `json:"password_authentication_disabled,omitempty"`
	TCPKeepAliveEnabled            bool                          `json:"tcp_keep_alive_enabled,omitempty"`
	PFS                            bool                          `json:"pfs,omitempty"`
	MTU                            uint32                        `json:"mtu,omitempty"`
	BaseMTU                        uint32                        `json:"base_mtu,omitempty"`
	DPDInterval                    badoption.Duration            `json:"dpd_interval,omitempty"`
	ReconnectTimeout               badoption.Duration            `json:"reconnect_timeout,omitempty"`
	TrojanInterval                 badoption.Duration            `json:"trojan_interval,omitempty"`
	QueueLength                    uint32                        `json:"queue_length,omitempty"`
	AllowInsecureCrypto            bool                          `json:"allow_insecure_crypto,omitempty"`
	TLS                            OpenConnectTLSOptions         `json:"tls,omitempty"`
	FormEntries                    []OpenConnectFormEntryOptions `json:"form_entries,omitempty"`
}

type OpenConnectTokenOptions struct {
	Mode       string `json:"mode,omitempty"`
	Secret     string `json:"secret,omitempty"`
	SecretPath string `json:"secret_path,omitempty"`
	PIN        string `json:"pin,omitempty"`
	Password   string `json:"password,omitempty"`
	DeviceID   string `json:"device_id,omitempty"`
	Counter    uint64 `json:"counter,omitempty"`
}

type OpenConnectMobileOptions struct {
	PlatformVersion string `json:"platform_version"`
	DeviceType      string `json:"device_type"`
	DeviceUniqueID  string `json:"device_unique_id"`
}

type OpenConnectCSDOptions struct {
	WrapperPath string `json:"wrapper_path,omitempty"`
}

type OpenConnectHIPOptions struct {
	WrapperPath string `json:"wrapper_path,omitempty"`
}

type OpenConnectTNCCOptions struct {
	WrapperPath                  string                              `json:"wrapper_path,omitempty"`
	DeviceID                     string                              `json:"device_id,omitempty"`
	UserAgent                    string                              `json:"user_agent,omitempty"`
	MachineIdentificationEnabled bool                                `json:"machine_identification_enabled,omitempty"`
	Certificates                 []OpenConnectTNCCCertificateOptions `json:"certificates,omitempty"`
}

type OpenConnectTNCCCertificateOptions struct {
	Certificate     badoption.Listable[string] `json:"certificate,omitempty"`
	CertificatePath string                     `json:"certificate_path,omitempty"`
}

type OpenConnectTLSOptions struct {
	Insecure                 bool                       `json:"insecure,omitempty"`
	ServerName               string                     `json:"server_name,omitempty"`
	PeerFingerprint          badoption.Listable[string] `json:"peer_fingerprint,omitempty"`
	SystemTrustDisabled      bool                       `json:"system_trust_disabled,omitempty"`
	CertificateAuthority     badoption.Listable[string] `json:"certificate_authority,omitempty"`
	CertificateAuthorityPath string                     `json:"certificate_authority_path,omitempty"`
	ClientCertificate        badoption.Listable[string] `json:"client_certificate,omitempty"`
	ClientCertificatePath    string                     `json:"client_certificate_path,omitempty"`
	ClientKey                badoption.Listable[string] `json:"client_key,omitempty"`
	ClientKeyPath            string                     `json:"client_key_path,omitempty"`
	ClientKeyPassword        string                     `json:"client_key_password,omitempty"`
	MCACertificate           badoption.Listable[string] `json:"mca_certificate,omitempty"`
	MCACertificatePath       string                     `json:"mca_certificate_path,omitempty"`
	MCAKey                   badoption.Listable[string] `json:"mca_key,omitempty"`
	MCAKeyPath               string                     `json:"mca_key_path,omitempty"`
	MCAKeyPassword           string                     `json:"mca_key_password,omitempty"`
}

type OpenConnectFormEntryOptions struct {
	FormID        string `json:"form_id,omitempty"`
	SubmissionKey string `json:"submission_key,omitempty"`
	Name          string `json:"name,omitempty"`
	Value         string `json:"value,omitempty"`
	Promote       bool   `json:"promote,omitempty"`
}

type OpenConnectDNSServerOptions struct {
	Endpoint               string `json:"endpoint,omitempty"`
	AcceptDefaultResolvers bool   `json:"accept_default_resolvers,omitempty"`
	AcceptSearchDomain     bool   `json:"accept_search_domain,omitempty"`
}
