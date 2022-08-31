//go:build !linux && !darwin

package redir

import (
	"net"
	"net/netip"
	"os"
)

func GetOriginalDestination(conn net.Conn) (destination netip.AddrPort, err error) {
	return netip.AddrPort{}, os.ErrInvalid
}
