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

type TrafficController interface {
	RoutedConnection(ctx context.Context, conn net.Conn, metadata InboundContext, matchedRule Rule) net.Conn
	RoutedPacketConnection(ctx context.Context, conn N.PacketConn, metadata InboundContext, matchedRule Rule) N.PacketConn
}
