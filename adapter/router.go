package adapter

import (
	"context"
	"net"
	"net/netip"

	N "github.com/sagernet/sing/common/network"

	"github.com/sagernet/sing-box/common/geoip"
	C "github.com/sagernet/sing-box/constant"

	"golang.org/x/net/dns/dnsmessage"
)

type Router interface {
	Service
	Outbound(tag string) (Outbound, bool)
	DefaultOutbound(network string) Outbound
	RouteConnection(ctx context.Context, conn net.Conn, metadata InboundContext) error
	RoutePacketConnection(ctx context.Context, conn N.PacketConn, metadata InboundContext) error
	GeoIPReader() *geoip.Reader
	LoadGeosite(code string) (Rule, error)
	Exchange(ctx context.Context, message *dnsmessage.Message) (*dnsmessage.Message, error)
	Lookup(ctx context.Context, domain string, strategy C.DomainStrategy) ([]netip.Addr, error)
	LookupDefault(ctx context.Context, domain string) ([]netip.Addr, error)
}

type Rule interface {
	Service
	UpdateGeosite() error
	Match(metadata *InboundContext) bool
	Outbound() string
	String() string
}
