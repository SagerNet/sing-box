package dialer

import (
	"net"

	"github.com/sagernet/sing/common/control"
)

type WireGuardListener interface {
	ListenPacketCompat(network, address string) (net.PacketConn, error)
}

var WgControlFns []control.Func
