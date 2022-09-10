package adapter

import (
	"context"
	"net"

	N "github.com/sagernet/sing/common/network"
)

type ClashServer interface {
	Service
	Mode() string
	RoutedConnection(ctx context.Context, conn net.Conn, metadata InboundContext, matchedRule Rule) (net.Conn, Tracker)
	RoutedPacketConnection(ctx context.Context, conn N.PacketConn, metadata InboundContext, matchedRule Rule) (N.PacketConn, Tracker)
}

type Tracker interface {
	Leave()
}

type OutboundGroup interface {
	Now() string
	All() []string
}

func OutboundTag(detour Outbound) string {
	if group, isGroup := detour.(OutboundGroup); isGroup {
		return group.Now()
	}
	return detour.Tag()
}
