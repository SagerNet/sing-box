package adapter

import (
	"context"
	"net"

	N "github.com/sagernet/sing/common/network"
)

type ClashServer interface {
	Service
	TrafficController
}

type Tracker interface {
	Leave()
}

type TrafficController interface {
	RoutedConnection(ctx context.Context, conn net.Conn, metadata InboundContext, matchedRule Rule) (net.Conn, Tracker)
	RoutedPacketConnection(ctx context.Context, conn N.PacketConn, metadata InboundContext, matchedRule Rule) (N.PacketConn, Tracker)
}

type OutboundGroup interface {
	Now() string
	All() []string
}
