package adapter

import (
	"context"
	"net"

	N "github.com/sagernet/sing/common/network"

	"github.com/sagernet/sing-box/common/geoip"
	"github.com/sagernet/sing-box/common/geosite"
)

type Router interface {
	Service
	Outbound(tag string) (Outbound, bool)
	RouteConnection(ctx context.Context, conn net.Conn, metadata InboundContext) error
	RoutePacketConnection(ctx context.Context, conn N.PacketConn, metadata InboundContext) error
	GeoIPReader() *geoip.Reader
	GeositeReader() *geosite.Reader
}

type Rule interface {
	Service
	UpdateGeosite() error
	Match(metadata *InboundContext) bool
	Outbound() string
	String() string
}
