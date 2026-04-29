package adapter

import (
	"net"
	"net/netip"
)

type NeighborEntry struct {
	Address    netip.Addr
	MACAddress net.HardwareAddr
	Hostname   string
}

type NeighborResolver interface {
	LookupMAC(address netip.Addr) (net.HardwareAddr, bool)
	LookupHostname(address netip.Addr) (string, bool)
	LookupAddresses(hostname string) []netip.Addr
	Start() error
	Close() error
}

type NeighborUpdateListener interface {
	UpdateNeighborTable(entries []NeighborEntry)
}
