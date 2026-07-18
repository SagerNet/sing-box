package option

import (
	"net/netip"

	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/json/badoption"
)

type OpenVPNEndpointOptions struct {
	System       bool           `json:"system,omitempty"`
	Name         string         `json:"name,omitempty"`
	MTU          uint32         `json:"mtu,omitempty"`
	UDPMapping   UDPNATBehavior `json:"udp_mapping,omitempty"`
	UDPFiltering UDPNATBehavior `json:"udp_filtering,omitempty"`
	UDPNATMax    uint32         `json:"udp_nat_max,omitempty"`
}

type OpenVPNClientEndpointOptions struct {
	DialerOptions
	ServerOptions
	OpenVPNEndpointOptions
	Network              string                           `json:"network,omitempty"`
	Servers              []OpenVPNRemoteOptions           `json:"servers,omitempty"`
	RemoteRandom         bool                             `json:"remote_random,omitempty"`
	Username             string                           `json:"username,omitempty"`
	Password             string                           `json:"password,omitempty"`
	AuthRetry            string                           `json:"auth_retry,omitempty"`
	StaticChallenge      string                           `json:"static_challenge,omitempty"`
	StaticChallengeEcho  bool                             `json:"static_challenge_echo,omitempty"`
	TLS                  *OpenVPNOutboundTLSOptions       `json:"tls,omitempty"`
	DataCiphers          badoption.Listable[string]       `json:"data_ciphers,omitempty"`
	DataCiphersFallback  string                           `json:"data_ciphers_fallback,omitempty"`
	Auth                 string                           `json:"auth,omitempty"`
	MSSFix               uint32                           `json:"mss_fix,omitempty"`
	Fragment             uint32                           `json:"fragment,omitempty"`
	Compression          string                           `json:"compression,omitempty"`
	CompressionLZO       string                           `json:"compression_lzo,omitempty"`
	AllowCompression     string                           `json:"allow_compression,omitempty"`
	RouteNoPull          bool                             `json:"route_no_pull,omitempty"`
	PullFilters          []OpenVPNPullFilterOptions       `json:"pull_filters,omitempty"`
	Routes               badoption.Listable[netip.Prefix] `json:"routes,omitempty"`
	RouteGateway         *badoption.Addr                  `json:"route_gateway,omitempty"`
	RouteMetric          int                              `json:"route_metric,omitempty"`
	RedirectGateway      bool                             `json:"redirect_gateway,omitempty"`
	RedirectGatewayFlags badoption.Listable[string]       `json:"redirect_gateway_flags,omitempty"`
	PingInterval         badoption.Duration               `json:"ping_interval,omitempty"`
	PingRestart          badoption.Duration               `json:"ping_restart,omitempty"`
	RenegotiateInterval  badoption.Duration               `json:"renegotiate_interval,omitempty"`
	ExplicitExitNotify   uint32                           `json:"explicit_exit_notify,omitempty"`
	UDPTimeout           UDPTimeoutCompat                 `json:"udp_timeout,omitempty"`
}

type OpenVPNServerEndpointOptions struct {
	ListenOptions
	OpenVPNEndpointOptions
	Network             string                           `json:"network,omitempty"`
	MaxClients          int                              `json:"max_clients,omitempty"`
	Address             badoption.Listable[netip.Prefix] `json:"address"`
	Topology            string                           `json:"topology,omitempty"`
	DuplicateCN         bool                             `json:"duplicate_cn,omitempty"`
	Users               []auth.User                      `json:"users,omitempty"`
	TLS                 *OpenVPNInboundTLSOptions        `json:"tls,omitempty"`
	DataCiphers         badoption.Listable[string]       `json:"data_ciphers,omitempty"`
	DataCiphersFallback string                           `json:"data_ciphers_fallback,omitempty"`
	Auth                string                           `json:"auth,omitempty"`
	Push                *OpenVPNPushOptions              `json:"push,omitempty"`
	PingInterval        badoption.Duration               `json:"ping_interval,omitempty"`
	PingRestart         badoption.Duration               `json:"ping_restart,omitempty"`
	RenegotiateInterval badoption.Duration               `json:"renegotiate_interval,omitempty"`
	HandshakeWindow     badoption.Duration               `json:"handshake_window,omitempty"`
}

type OpenVPNRemoteOptions struct {
	ServerOptions
	Network string `json:"network,omitempty"`
}

type OpenVPNPullFilterOptions struct {
	Action string `json:"action"`
	Text   string `json:"text"`
}

type OpenVPNOutboundTLSOptions struct {
	ServerName            string                     `json:"server_name,omitempty"`
	ServerNameType        string                     `json:"server_name_type,omitempty"`
	Certificate           badoption.Listable[string] `json:"certificate,omitempty"`
	CertificatePath       string                     `json:"certificate_path,omitempty"`
	ClientCertificate     badoption.Listable[string] `json:"client_certificate,omitempty"`
	ClientCertificatePath string                     `json:"client_certificate_path,omitempty"`
	ClientKey             badoption.Listable[string] `json:"client_key,omitempty"`
	ClientKeyPath         string                     `json:"client_key_path,omitempty"`
	PeerFingerprint       badoption.Listable[string] `json:"peer_fingerprint,omitempty"`
	CRLPath               string                     `json:"crl_path,omitempty"`
	RemoteCertificateKU   badoption.Listable[string] `json:"remote_certificate_ku,omitempty"`
	RemoteCertificateEKU  string                     `json:"remote_certificate_eku,omitempty"`
	VersionMin            string                     `json:"version_min,omitempty"`
	VersionMax            string                     `json:"version_max,omitempty"`
	Cipher                string                     `json:"cipher,omitempty"`
	Groups                string                     `json:"groups,omitempty"`
	ControlWrap           *OpenVPNControlWrapOptions `json:"control_wrap,omitempty"`
}

type OpenVPNInboundTLSOptions struct {
	Certificate             badoption.Listable[string]        `json:"certificate,omitempty"`
	CertificatePath         string                            `json:"certificate_path,omitempty"`
	Key                     badoption.Listable[string]        `json:"key,omitempty"`
	KeyPath                 string                            `json:"key_path,omitempty"`
	ClientCertificate       badoption.Listable[string]        `json:"client_certificate,omitempty"`
	ClientCertificatePath   string                            `json:"client_certificate_path,omitempty"`
	VerifyClientCertificate string                            `json:"verify_client_certificate,omitempty"`
	ControlWrap             *OpenVPNInboundControlWrapOptions `json:"control_wrap,omitempty"`
}

type OpenVPNControlWrapOptions struct {
	Type      string                     `json:"type,omitempty"`
	Key       badoption.Listable[string] `json:"key,omitempty"`
	KeyPath   string                     `json:"key_path,omitempty"`
	Direction string                     `json:"direction,omitempty"`
}

type OpenVPNInboundControlWrapOptions struct {
	Type        string                     `json:"type,omitempty"`
	Key         badoption.Listable[string] `json:"key,omitempty"`
	KeyPath     string                     `json:"key_path,omitempty"`
	Direction   string                     `json:"direction,omitempty"`
	ForceCookie bool                       `json:"force_cookie,omitempty"`
}

type OpenVPNPushOptions struct {
	Routes               badoption.Listable[netip.Prefix] `json:"routes,omitempty"`
	DNS                  badoption.Listable[netip.Addr]   `json:"dns,omitempty"`
	RedirectGateway      bool                             `json:"redirect_gateway,omitempty"`
	RedirectGatewayFlags badoption.Listable[string]       `json:"redirect_gateway_flags,omitempty"`
	BlockOutsideDNS      bool                             `json:"block_outside_dns,omitempty"`
	PingInterval         badoption.Duration               `json:"ping_interval,omitempty"`
	PingRestart          badoption.Duration               `json:"ping_restart,omitempty"`
}
