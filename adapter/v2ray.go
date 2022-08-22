package adapter

import (
	"context"
	"net"
)

type V2RayServerTransport interface {
	Network() []string
	Serve(listener net.Listener) error
	ServePacket(listener net.PacketConn) error
	Close() error
}

type V2RayClientTransport interface {
	DialContext(ctx context.Context) (net.Conn, error)
}
