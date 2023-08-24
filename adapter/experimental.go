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
	ModeList() []string
	StoreSelected() bool
	StoreFakeIP() bool
	CacheFile() ClashCacheFile
	HistoryStorage() *urltest.HistoryStorage
	RoutedConnection(ctx context.Context, conn net.Conn, metadata InboundContext, matchedRule Rule) (net.Conn, Tracker)
	RoutedPacketConnection(ctx context.Context, conn N.PacketConn, metadata InboundContext, matchedRule Rule) (N.PacketConn, Tracker)
}

type ClashCacheFile interface {
	LoadMode() string
	StoreMode(mode string) error
	LoadSelected(group string) string
	StoreSelected(group string, selected string) error
	LoadGroupExpand(group string) (isExpand bool, loaded bool)
	StoreGroupExpand(group string, expand bool) error
	FakeIPStorage
}

type Tracker interface {
	Leave()
}

type OutboundGroup interface {
	Outbound
	Now() string
	All() []string
}

type URLTestGroup interface {
	OutboundGroup
	URLTest(ctx context.Context, url string) (map[string]uint16, error)
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
