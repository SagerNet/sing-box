package constant

const (
	DefaultDNSTTL = 600
)

type DomainStrategy = uint8

const (
	DomainStrategyAsIS DomainStrategy = iota
	DomainStrategyPreferIPv4
	DomainStrategyPreferIPv6
	DomainStrategyIPv4Only
	DomainStrategyIPv6Only
)

const (
	DNSTypeLegacy     = "legacy"
	DNSTypeUDP        = "udp"
	DNSTypeTCP        = "tcp"
	DNSTypeTLS        = "tls"
	DNSTypeHTTPS      = "https"
	DNSTypeQUIC       = "quic"
	DNSTypeHTTP3      = "h3"
	DNSTypeHosts      = "hosts"
	DNSTypeLocal      = "local"
	DNSTypePreDefined = "predefined"
	DNSTypeFakeIP     = "fakeip"
	DNSTypeDHCP       = "dhcp"
)

const (
	DNSProviderAliDNS     = "alidns"
	DNSProviderCloudflare = "cloudflare"
)
