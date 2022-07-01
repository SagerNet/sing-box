package adapter

import (
	"context"
	"net"

	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type Outbound interface {
	Type() string
	Tag() string
	NewConnection(ctx context.Context, conn net.Conn, destination M.Socksaddr) error
	NewPacketConnection(ctx context.Context, conn N.PacketConn, destination M.Socksaddr) error
	N.Dialer
}
