package adapter

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/common/urltest"
	N "github.com/sagernet/sing/common/network"
)

type ClashServer interface {
	Service
	PreStarter
	Mode() string
	StoreSelected() bool
	StoreFakeIP() bool
	CacheFile() ClashCacheFile
	HistoryStorage() *urltest.HistoryStorage
	RoutedConnection(ctx context.Context, conn net.Conn, metadata InboundContext, matchedRule Rule) (net.Conn, Tracker)
	RoutedPacketConnection(ctx context.Context, conn N.PacketConn, metadata InboundContext, matchedRule Rule) (N.PacketConn, Tracker)
}

type ClashCacheFile interface {
	LoadSelected(group string) string
	StoreSelected(group string, selected string) error
	FakeIPStorage
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

type V2RayServer interface {
	Service
	StatsService() V2RayStatsService
}

type V2RayStatsService interface {
	RoutedConnection(inbound string, outbound string, user string, conn net.Conn) net.Conn
	RoutedPacketConnection(inbound string, outbound string, user string, conn N.PacketConn) N.PacketConn
}
