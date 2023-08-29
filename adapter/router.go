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
	"github.com/sagernet/sing/service"

	mdns "github.com/miekg/dns"
)

type Router interface {
	Service

	Outbounds() []Outbound
	Outbound(tag string) (Outbound, bool)
	DefaultOutbound(network string) Outbound

	FakeIPStore() FakeIPStore

	RouteConnection(ctx context.Context, conn net.Conn, metadata InboundContext) error
	RoutePacketConnection(ctx context.Context, conn N.PacketConn, metadata InboundContext) error

	GeoIPReader() *geoip.Reader
	LoadGeosite(code string) (Rule, error)

	Exchange(ctx context.Context, message *mdns.Msg) (*mdns.Msg, error)
	Lookup(ctx context.Context, domain string, strategy dns.DomainStrategy) ([]netip.Addr, error)
	LookupDefault(ctx context.Context, domain string) ([]netip.Addr, error)
	ClearDNSCache()

	InterfaceFinder() control.InterfaceFinder
	UpdateInterfaces() error
	DefaultInterface() string
	AutoDetectInterface() bool
	AutoDetectInterfaceFunc() control.Func
	DefaultMark() int
	NetworkMonitor() tun.NetworkUpdateMonitor
	InterfaceMonitor() tun.DefaultInterfaceMonitor
	PackageManager() tun.PackageManager
	Rules() []Rule

	TimeService

	ClashServer() ClashServer
	SetClashServer(server ClashServer)

	V2RayServer() V2RayServer
	SetV2RayServer(server V2RayServer)

	ResetNetwork() error
}

func ContextWithRouter(ctx context.Context, router Router) context.Context {
	return service.ContextWith(ctx, router)
}

func RouterFromContext(ctx context.Context) Router {
	return service.FromContext[Router](ctx)
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
	RewriteTTL() *uint32
}

type InterfaceUpdateListener interface {
	InterfaceUpdated()
}
