package adapter

import (
	"context"
	"net"

	N "github.com/sagernet/sing/common/network"
)

type Router interface {
	DefaultOutbound() Outbound
	Outbound(tag string) (Outbound, bool)
	RouteConnection(ctx context.Context, conn net.Conn, metadata InboundContext) error
	RoutePacketConnection(ctx context.Context, conn N.PacketConn, metadata InboundContext) error
	Close() error
}

type Rule interface {
	Match(metadata InboundContext) bool
	Outbound() string
	String() string
}
