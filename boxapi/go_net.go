package boxapi

import (
	"context"
	"net"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing/common/metadata"
)

func DialContext(ctx context.Context, box *box.Box, tracker adapter.ConnectionTracker, network, addr string) (net.Conn, error) {
	defOutboundTag := box.Outbound().Default().Tag()
	conn, err := dialer.NewDetour(box.Outbound(), defOutboundTag, true).DialContext(ctx, network, metadata.ParseSocksaddr(addr))
	if err != nil {
		return nil, err
	}
	if ss, ok := tracker.(*SbStatsService); ok {
		conn = ss.RoutedConnectionInternal("", defOutboundTag, "", conn, false)
	}
	return conn, nil
}
