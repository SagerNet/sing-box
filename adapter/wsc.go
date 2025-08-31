package adapter

import (
	"context"
	"net"
)

type WSCServerTransport interface {
}

type WSCClientTransport interface {
	DialContext(ctx context.Context, network string, endpoint string) (net.Conn, error)
	Close(ctx context.Context) error
}
