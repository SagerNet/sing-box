//go:build !linux

package redir

import (
	"net"
	"net/netip"
	"os"
)

func TProxy(fd uintptr, isIPv6 bool) error {
	return os.ErrInvalid
}

func TProxyUDP(fd uintptr, isIPv6 bool) error {
	return os.ErrInvalid
}

func GetOriginalDestinationFromOOB(oob []byte) (netip.AddrPort, error) {
	return netip.AddrPort{}, os.ErrInvalid
}

func DialUDP(lAddr *net.UDPAddr, rAddr *net.UDPAddr) (*net.UDPConn, error) {
	return nil, os.ErrInvalid
}
