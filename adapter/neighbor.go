package adapter

import (
	"net"
	"net/netip"
)

type NeighborResolver interface {
	LookupMAC(address netip.Addr) (net.HardwareAddr, bool)
	LookupHostname(address netip.Addr) (string, bool)
	Start() error
	Close() error
}
