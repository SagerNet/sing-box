package adapter

import (
	"context"
	"net"

	"github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/network"
)

type WSCServerTransport interface {
	Network() []string
	Serve(listener net.Listener) error
	ServePacket(listener net.PacketConn) error
	Close() error
}

type WSCServerTransportHandler interface {
	network.TCPConnectionHandler
	network.UDPConnectionHandler
	exceptions.Handler
}

type WSCClientTransport interface {
	DialContext(ctx context.Context, network string, endpoint string) (net.Conn, error)
	ListenPacket(ctx context.Context, network string, endpoint string) (net.PacketConn, error)
	Close(ctx context.Context) error
}
