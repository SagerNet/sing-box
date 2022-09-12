package adapter

import (
	"context"
	"net"
	"net/netip"

	"github.com/sagernet/sing-box/common/geoip"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/control"
	N "github.com/sagernet/sing/common/network"

	"golang.org/x/net/dns/dnsmessage"
)

type Router interface {
	Service

	Outbounds() []Outbound
	Outbound(tag string) (Outbound, bool)
	AddOutbound(string, Outbound)
	DefaultOutbound(network string) Outbound

	RouteConnection(ctx context.Context, conn net.Conn, metadata InboundContext) error
	RoutePacketConnection(ctx context.Context, conn N.PacketConn, metadata InboundContext) error

	GeoIPReader() *geoip.Reader
	LoadGeosite(code string) (Rule, error)

	Exchange(ctx context.Context, message *dnsmessage.Message) (*dnsmessage.Message, error)
	Lookup(ctx context.Context, domain string, strategy dns.DomainStrategy) ([]netip.Addr, error)
	LookupDefault(ctx context.Context, domain string) ([]netip.Addr, error)

	InterfaceBindManager() control.BindManager
	DefaultInterface() string
	AutoDetectInterface() bool
	DefaultMark() int
	NetworkMonitor() tun.NetworkUpdateMonitor
	InterfaceMonitor() tun.DefaultInterfaceMonitor
	PackageManager() tun.PackageManager
	Rules() []Rule

	ClashServer() ClashServer
	SetClashServer(controller ClashServer)
}

type Rule interface {
	Service
	Type() string
	UpdateGeosite() error
	Match(metadata *InboundContext) bool
	Outbound() string
	String() string
}

type DNSRule interface {
	Rule
	DisableCache() bool
}
