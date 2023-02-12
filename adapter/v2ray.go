package adapter

import (
	"context"
	"net"

	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type V2RayServerTransport interface {
	Network() []string
	Serve(listener net.Listener) error
	ServePacket(listener net.PacketConn) error
	Close() error
}

type V2RayServerTransportHandler interface {
	N.TCPConnectionHandler
	E.Handler
	FallbackConnection(ctx context.Context, conn net.Conn, metadata M.Metadata) error
}

type V2RayClientTransport interface {
	DialContext(ctx context.Context) (net.Conn, error)
}
