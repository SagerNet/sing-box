package openconnect

import (
	"net/netip"
	"time"

	"github.com/sagernet/sing-openconnect"
)

const (
	DefaultMTU     = 1500
	PacketHeadroom = openconnect.PacketHeadroom
)

type Configuration struct {
	MTU                      uint32
	Addresses                []netip.Prefix
	Routes                   []Route
	ExcludedRoutes           []Route
	DNS                      []netip.Addr
	NBNS                     []netip.Addr
	SearchDomains            []string
	SplitDNS                 []string
	SplitDNSRules            []SplitDNSRule
	ProxyAutoConfigURL       string
	Banner                   string
	TunnelAllDNS             bool
	ClientBypassProtocol     bool
	IdleTimeout              time.Duration
	AuthenticationExpiration time.Time
}

type Route struct {
	Prefix  netip.Prefix
	Gateway netip.Addr
	Metric  int
}

type SplitDNSRule struct {
	Domains []string
	Servers []netip.Addr
}
