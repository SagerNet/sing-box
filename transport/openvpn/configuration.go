package openvpn

import "net/netip"

const (
	DefaultMTU     = 1500
	PacketHeadroom = 4096
)

type Configuration struct {
	MTU            uint32
	Address        []netip.Prefix
	Routes         []Route
	ExcludedRoutes []Route
	DNS            []netip.Addr
	DNSServers     []DNSServer
	SearchDomains  []string
	DNSRoutes      []string
	Topology       string
	Interface      string
	BlockIPv6      bool
}

type Route struct {
	Prefix  netip.Prefix
	Gateway netip.Addr
	Metric  int
}

type DNSServer struct {
	Priority       int
	Addresses      []netip.AddrPort
	ResolveDomains []string
	DNSSEC         string
	Transport      string
	SNI            string
}
