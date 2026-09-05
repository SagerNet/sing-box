//go:build !with_gvisor

package geph

import (
	"context"
	"net"

	M "github.com/sagernet/sing/common/metadata"
)

type packetStack interface {
	DialContext(context.Context, string, M.Socksaddr) (net.Conn, error)
	ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error)
	Close() error
}

func newPacketStack(<-chan []byte, func([]byte) error) (packetStack, error) {
	return nil, errGVisorRequired
}

var errGVisorRequired = unsupportedError("Geph endpoint requires a build with -tags with_gvisor")

type unsupportedError string

func (e unsupportedError) Error() string { return string(e) }
