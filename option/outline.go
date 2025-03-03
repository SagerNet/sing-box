package option

import "time"

// OutboundOutlineOptions set the outbound options used by the outline-sdk
// smart dialer. You can find more details about the parameters by looking
// through the implementation: https://github.com/Jigsaw-Code/outline-sdk/blob/v0.0.18/x/smart/stream_dialer.go#L65-L100
// Or check the documentation README: https://github.com/Jigsaw-Code/outline-sdk/tree/v0.0.18/x/smart
type OutboundOutlineOptions struct {
	DialerOptions
	DNSResolvers []DNSEntryConfig `json:"dns,omitempty" yaml:"dns,omitempty"`
	TLS          []string         `json:"tls,omitempty" yaml:"tls,omitempty"`
	TestTimeout  *time.Duration   `json:"test_timeout" yaml:"-"`
	Domains      []string         `json:"domains" yaml:"-"`
}

// DNSEntryConfig specifies a list of resolvers to test and they can be one of
// the attributes (system, https, tls, udp or tcp)
type DNSEntryConfig struct {
	// System is used for using the system as a resolver, if you want to use it
	// provide an empty object.
	System *struct{} `json:"system,omitempty"`
	// HTTPS use an encrypted DNS over HTTPS (DoH) resolver.
	HTTPS *HTTPSEntryConfig `json:"https,omitempty"`
	// TLS use an encrypted DNS over TLS (DoT) resolver.
	TLS *TLSEntryConfig `json:"tls,omitempty"`
	// UDP use a UDP resolver
	UDP *UDPEntryConfig `json:"udp,omitempty"`
	// TCP use a TCP resolver
	TCP *TCPEntryConfig `json:"tcp,omitempty"`
}

type HTTPSEntryConfig struct {
	// Domain name of the host.
	Name string `json:"name,omitempty"`
	// Host:port. Defaults to Name:443.
	Address string `json:"address,omitempty"`
}

type TLSEntryConfig struct {
	// Domain name of the host.
	Name string `json:"name,omitempty"`
	// Host:port. Defaults to Name:853.
	Address string `json:"address,omitempty"`
}

type UDPEntryConfig struct {
	// Host:port.
	Address string `json:"address,omitempty"`
}

type TCPEntryConfig struct {
	// Host:port.
	Address string `json:"address,omitempty"`
}
