//go:build !darwin && !linux && !windows

package route

import (
	"net"
	"net/netip"
)

func systemGateways(interfaceIndex int) []netip.Addr {
	return nil
}

func systemNeighborHardwareAddresses(interfaceIndex int, addresses []netip.Addr) map[netip.Addr]net.HardwareAddr {
	return nil
}
