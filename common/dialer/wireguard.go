package dialer

import (
	"net"
)

type WireGuardListener interface {
	ListenPacketCompat(network, address string) (net.PacketConn, error)
}
