package dialer

import (
	"github.com/sagernet/sing/common/control"
	"net"

	_ "github.com/redpilllabs/wireguard-go/conn"
)

type WireGuardListener interface {
	ListenPacketCompat(network, address string) (net.PacketConn, error)
}

var WgControlFns []control.Func
