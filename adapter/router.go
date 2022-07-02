package adapter

import (
	"context"
	"net"

	"github.com/oschwald/geoip2-golang"
	N "github.com/sagernet/sing/common/network"
)

type Router interface {
	Start() error
	Close() error

	DefaultOutbound() Outbound
	Outbound(tag string) (Outbound, bool)
	RouteConnection(ctx context.Context, conn net.Conn, metadata InboundContext) error
	RoutePacketConnection(ctx context.Context, conn N.PacketConn, metadata InboundContext) error
	GeoIPReader() *geoip2.Reader
}

type Rule interface {
	Match(metadata *InboundContext) bool
	Outbound() string
	String() string
}
