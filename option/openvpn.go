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
	Mode                 string                           `json:"mode,omitempty" enum:"tls,static_key"`
	Network              string                           `json:"network,omitempty" enum:"udp,udp4,udp6,tcp,tcp4,tcp6"`
	Servers              []OpenVPNRemoteOptions           `json:"servers,omitempty"`
	RemoteRandom         bool                             `json:"remote_random,omitempty"`
	Address              badoption.Listable[netip.Prefix] `json:"address,omitempty"`
	PeerAddress          badoption.Addr                   `json:"peer_address,omitempty"`
	PeerAddressIPv6      badoption.Addr                   `json:"peer_address_ipv6,omitempty"`
	Topology             string                           `json:"topology,omitempty" enum:"net30,p2p,subnet"`
	Username             string                           `json:"username,omitempty"`
	Password             string                           `json:"password,omitempty"`
	AuthRetry            string                           `json:"auth_retry,omitempty" enum:"none,nointeract,interact"`
	StaticChallenge      string                           `json:"static_challenge,omitempty"`
	StaticChallengeEcho  bool                             `json:"static_challenge_echo,omitempty"`
	StaticKey            badoption.Listable[string]       `json:"static_key,omitempty"`
	StaticKeyPath        string                           `json:"static_key_path,omitempty"`
	KeyDirection         string                           `json:"key_direction,omitempty" enum:"server,client"`
	TLS                  *OpenVPNOutboundTLSOptions       `json:"tls,omitempty"`
	Cipher               string                           `json:"cipher,omitempty"`
	DataCiphers          badoption.Listable[string]       `json:"data_ciphers,omitempty"`
	DataCiphersFallback  string                           `json:"data_ciphers_fallback,omitempty"`
	Auth                 string                           `json:"auth,omitempty"`
	MSSFix               uint32                           `json:"mss_fix,omitempty"`
	MSSFixDisabled       bool                             `json:"mss_fix_disabled,omitempty"`
	MSSFixMode           string                           `json:"mss_fix_mode,omitempty" enum:"mtu,fixed"`
	Fragment             uint32                           `json:"fragment,omitempty"`
	ReplayWindow         uint32                           `json:"replay_window,omitempty"`
	ReplayWindowTime     badoption.Duration               `json:"replay_window_time,omitempty"`
	Compression          string                           `json:"compression,omitempty" enum:"none,no,lz4,lz4-v2,stub,stub-v2,disabled,off"`
	CompressionLZO       string                           `json:"compression_lzo,omitempty" enum:"none,no,yes,adaptive,asym,disabled,off"`
	AllowCompression     string                           `json:"allow_compression,omitempty" enum:"no,asym,yes"`
	RouteNoPull          bool                             `json:"route_no_pull,omitempty"`
	PullFilters          []OpenVPNPullFilterOptions       `json:"pull_filters,omitempty"`
	Routes               badoption.Listable[netip.Prefix] `json:"routes,omitempty"`
	RouteGateway         *badoption.Addr                  `json:"route_gateway,omitempty"`
	RouteMetric          int                              `json:"route_metric,omitempty"`
	RedirectGateway      bool                             `json:"redirect_gateway,omitempty"`
	RedirectGatewayFlags badoption.Listable[string]       `json:"redirect_gateway_flags,omitempty"`
	RedirectPrivate      bool                             `json:"redirect_private,omitempty"`
	BlockIPv6            bool                             `json:"block_ipv6,omitempty"`
	PingInterval         badoption.Duration               `json:"ping_interval,omitempty"`
	PingRestart          badoption.Duration               `json:"ping_restart,omitempty"`
	PingRestartDisabled  bool                             `json:"ping_restart_disabled,omitempty"`
	RenegotiateInterval  badoption.Duration               `json:"renegotiate_interval,omitempty"`
	RenegotiateDisabled  bool                             `json:"renegotiate_disabled,omitempty"`
	RenegotiateBytes     uint64                           `json:"renegotiate_bytes,omitempty"`
	RenegotiatePackets   uint64                           `json:"renegotiate_packets,omitempty"`
	TLSTimeout           badoption.Duration               `json:"tls_timeout,omitempty"`
	HandshakeWindow      badoption.Duration               `json:"handshake_window,omitempty"`
	ExplicitExitNotify   uint32                           `json:"explicit_exit_notify,omitempty"`
	UDPTimeout           UDPTimeoutCompat                 `json:"udp_timeout,omitempty"`
}

type OpenVPNServerEndpointOptions struct {
	ListenOptions
	OpenVPNEndpointOptions
	Mode                string                           `json:"mode,omitempty" enum:"tls,static_key"`
	Network             string                           `json:"network,omitempty" enum:"tcp,udp"`
	Remote              string                           `json:"remote,omitempty"`
	RemotePort          uint16                           `json:"remote_port,omitempty"`
	MaxClients          int                              `json:"max_clients,omitempty"`
	Address             badoption.Listable[netip.Prefix] `json:"address"`
	PeerAddress         badoption.Addr                   `json:"peer_address,omitempty"`
	PeerAddressIPv6     badoption.Addr                   `json:"peer_address_ipv6,omitempty"`
	Topology            string                           `json:"topology,omitempty" enum:"net30,p2p,subnet"`
	DuplicateCN         bool                             `json:"duplicate_cn,omitempty"`
	Users               []auth.User                      `json:"users,omitempty"`
	StaticKey           badoption.Listable[string]       `json:"static_key,omitempty"`
	StaticKeyPath       string                           `json:"static_key_path,omitempty"`
	KeyDirection        string                           `json:"key_direction,omitempty" enum:"server,client"`
	TLS                 *OpenVPNInboundTLSOptions        `json:"tls,omitempty"`
	Cipher              string                           `json:"cipher,omitempty"`
	DataCiphers         badoption.Listable[string]       `json:"data_ciphers,omitempty"`
	DataCiphersFallback string                           `json:"data_ciphers_fallback,omitempty"`
	Auth                string                           `json:"auth,omitempty"`
	MSSFix              uint32                           `json:"mss_fix,omitempty"`
	MSSFixDisabled      bool                             `json:"mss_fix_disabled,omitempty"`
	MSSFixMode          string                           `json:"mss_fix_mode,omitempty" enum:"mtu,fixed"`
	ReplayWindow        uint32                           `json:"replay_window,omitempty"`
	ReplayWindowTime    badoption.Duration               `json:"replay_window_time,omitempty"`
	Push                *OpenVPNPushOptions              `json:"push,omitempty"`
	PingInterval        badoption.Duration               `json:"ping_interval,omitempty"`
	PingRestart         badoption.Duration               `json:"ping_restart,omitempty"`
	RenegotiateInterval badoption.Duration               `json:"renegotiate_interval,omitempty"`
	RenegotiateDisabled bool                             `json:"renegotiate_disabled,omitempty"`
	RenegotiateBytes    uint64                           `json:"renegotiate_bytes,omitempty"`
	RenegotiatePackets  uint64                           `json:"renegotiate_packets,omitempty"`
	HandshakeWindow     badoption.Duration               `json:"handshake_window,omitempty"`
}

type OpenVPNRemoteOptions struct {
	ServerOptions
	Network string `json:"network,omitempty" enum:"udp,udp4,udp6,tcp,tcp4,tcp6"`
}

type OpenVPNPullFilterOptions struct {
	Action string `json:"action" enum:"accept,ignore,reject"`
	Text   string `json:"text"`
}

type OpenVPNOutboundTLSOptions struct {
	ServerName            string                     `json:"server_name,omitempty"`
	ServerNameType        string                     `json:"server_name_type,omitempty" enum:"subject,name,name-prefix"`
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
	RemoteCertificateTLS  string                     `json:"remote_certificate_tls,omitempty" enum:"server,client,none"`
	CertificateProfile    string                     `json:"certificate_profile,omitempty" enum:"legacy,preferred,insecure,suiteb"`
	NSCertificateType     string                     `json:"ns_certificate_type,omitempty" enum:"server,client"`
	VersionMin            string                     `json:"version_min,omitempty" enum:"1.0,1.1,1.2,1.3"`
	VersionMax            string                     `json:"version_max,omitempty" enum:"1.0,1.1,1.2,1.3"`
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
	VerifyClientCertificate string                            `json:"verify_client_certificate,omitempty" enum:"require,optional,none"`
	ClientName              string                            `json:"client_name,omitempty"`
	ClientNameType          string                            `json:"client_name_type,omitempty" enum:"subject,name,name-prefix"`
	PeerFingerprint         badoption.Listable[string]        `json:"peer_fingerprint,omitempty"`
	CRLPath                 string                            `json:"crl_path,omitempty"`
	RemoteCertificateKU     badoption.Listable[string]        `json:"remote_certificate_ku,omitempty"`
	RemoteCertificateEKU    string                            `json:"remote_certificate_eku,omitempty"`
	RemoteCertificateTLS    string                            `json:"remote_certificate_tls,omitempty" enum:"server,client,none"`
	CertificateProfile      string                            `json:"certificate_profile,omitempty" enum:"legacy,preferred,insecure,suiteb"`
	NSCertificateType       string                            `json:"ns_certificate_type,omitempty" enum:"server,client"`
	VersionMin              string                            `json:"version_min,omitempty" enum:"1.0,1.1,1.2,1.3"`
	VersionMax              string                            `json:"version_max,omitempty" enum:"1.0,1.1,1.2,1.3"`
	Cipher                  string                            `json:"cipher,omitempty"`
	Groups                  string                            `json:"groups,omitempty"`
	ControlWrap             *OpenVPNInboundControlWrapOptions `json:"control_wrap,omitempty"`
}

type OpenVPNControlWrapOptions struct {
	Type      string                     `json:"type,omitempty" enum:"tls_auth,tls_crypt,tls_crypt_v2"`
	Key       badoption.Listable[string] `json:"key,omitempty"`
	KeyPath   string                     `json:"key_path,omitempty"`
	Direction string                     `json:"direction,omitempty" enum:"server,client"`
}

type OpenVPNInboundControlWrapOptions struct {
	Type        string                     `json:"type,omitempty" enum:"tls_auth,tls_crypt,tls_crypt_v2"`
	Key         badoption.Listable[string] `json:"key,omitempty"`
	KeyPath     string                     `json:"key_path,omitempty"`
	Direction   string                     `json:"direction,omitempty" enum:"server,client"`
	ForceCookie bool                       `json:"force_cookie,omitempty"`
}

type OpenVPNPushOptions struct {
	Routes               badoption.Listable[netip.Prefix] `json:"routes,omitempty"`
	DNS                  badoption.Listable[netip.Addr]   `json:"dns,omitempty"`
	DNSServers           []OpenVPNPushDNSServerOptions    `json:"dns_servers,omitempty"`
	SearchDomains        badoption.Listable[string]       `json:"search_domains,omitempty"`
	DHCPOptions          badoption.Listable[string]       `json:"dhcp_options,omitempty"`
	RedirectGateway      bool                             `json:"redirect_gateway,omitempty"`
	RedirectGatewayFlags badoption.Listable[string]       `json:"redirect_gateway_flags,omitempty"`
	BlockOutsideDNS      bool                             `json:"block_outside_dns,omitempty"`
	PingInterval         badoption.Duration               `json:"ping_interval,omitempty"`
	PingRestart          badoption.Duration               `json:"ping_restart,omitempty"`
}

type OpenVPNPushDNSServerOptions struct {
	Priority       int                        `json:"priority"`
	Addresses      badoption.Listable[string] `json:"addresses"`
	ResolveDomains badoption.Listable[string] `json:"resolve_domains,omitempty"`
	DNSSEC         string                     `json:"dnssec,omitempty" enum:"yes,optional,no"`
	Transport      string                     `json:"transport,omitempty" enum:"plain,dot,doh"`
	SNI            string                     `json:"sni,omitempty"`
}

type OpenVPNDNSServerOptions struct {
	Endpoint               string `json:"endpoint,omitempty"`
	AcceptDefaultResolvers bool   `json:"accept_default_resolvers,omitempty"`
	AcceptSearchDomain     bool   `json:"accept_search_domain,omitempty"`
}
