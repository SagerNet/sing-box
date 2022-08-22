package adapter

import (
	"context"
	"net"
)

type V2RayServerTransport interface {
	Serve(listener net.Listener) error
	Close() error
}

type V2RayClientTransport interface {
	DialContext(ctx context.Context) (net.Conn, error)
}
