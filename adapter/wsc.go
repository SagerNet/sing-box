package adapter

import (
	"context"
	"net"
)

type WSCServerTransport interface {
}

type WSCClientTransport interface {
	DialContext(ctx context.Context, network string, endpoint string) (net.Conn, error)
	ListenPacket(ctx context.Context, network string, endpoint string) (net.PacketConn, error)
	Close(ctx context.Context) error
}
