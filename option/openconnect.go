package option

import "github.com/sagernet/sing/common/json/badoption"

type OpenConnectEndpointOptions struct {
	DialerOptions
	System              bool                          `json:"system,omitempty"`
	Name                string                        `json:"name,omitempty"`
	UDPTimeout          badoption.Duration            `json:"udp_timeout,omitempty"`
	UDPMapping          UDPNATBehavior                `json:"udp_mapping,omitempty"`
	UDPFiltering        UDPNATBehavior                `json:"udp_filtering,omitempty"`
	UDPNATMax           uint32                        `json:"udp_nat_max,omitempty"`
	Server              string                        `json:"server"`
	Flavor              string                        `json:"flavor,omitempty"`
	Username            string                        `json:"username,omitempty"`
	Password            string                        `json:"password,omitempty"`
	AuthGroup           string                        `json:"auth_group,omitempty"`
	Token               *OpenConnectTokenOptions      `json:"token,omitempty"`
	ReportedOS          string                        `json:"reported_os,omitempty"`
	UserAgent           string                        `json:"user_agent,omitempty"`
	CSD                 *OpenConnectCSDOptions        `json:"csd,omitempty"`
	HIP                 *OpenConnectHIPOptions        `json:"hip,omitempty"`
	TNCC                *OpenConnectTNCCOptions       `json:"tncc,omitempty"`
	NoUDP               bool                          `json:"no_udp,omitempty"`
	AllowInsecureCrypto bool                          `json:"allow_insecure_crypto,omitempty"`
	TLS                 OpenConnectTLSOptions         `json:"tls,omitempty"`
	FormEntries         []OpenConnectFormEntryOptions `json:"form_entries,omitempty"`
}

type OpenConnectTokenOptions struct {
	Mode     string `json:"mode,omitempty"`
	Secret   string `json:"secret,omitempty"`
	PIN      string `json:"pin,omitempty"`
	Password string `json:"password,omitempty"`
	DeviceID string `json:"device_id,omitempty"`
	Counter  uint64 `json:"counter,omitempty"`
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
