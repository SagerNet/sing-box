package adapter

import (
	"context"
	"net"
	"net/netip"

	"github.com/sagernet/sing-box/common/geoip"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing/common/control"
	N "github.com/sagernet/sing/common/network"

	"golang.org/x/net/dns/dnsmessage"
)

type Router interface {
	Service

	Outbounds() []Outbound
	Outbound(tag string) (Outbound, bool)
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
	AutoDetectInterfaceName() string
	AutoDetectInterfaceIndex() int

	Rules() []Rule
	SetTrafficController(controller TrafficController)
}

type Rule interface {
	Service
	Type() string
	UpdateGeosite() error
	Match(metadata *InboundContext) bool
	Outbound() string
	String() string
}
