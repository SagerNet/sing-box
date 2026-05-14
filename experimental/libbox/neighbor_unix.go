//go:build linux || darwin

package libbox

import (
	"net"
	"net/netip"
)

func tableToIterator(table map[netip.Addr]net.HardwareAddr) NeighborEntryIterator {
	entries := make([]*NeighborEntry, 0, len(table))
	for address, mac := range table {
		entries = append(entries, &NeighborEntry{
			Address:    address.String(),
			MacAddress: mac.String(),
		})
	}
	return &neighborEntryIterator{entries}
}

type neighborEntryIterator struct {
	entries []*NeighborEntry
}

func (i *neighborEntryIterator) HasNext() bool {
	return len(i.entries) > 0
}

func (i *neighborEntryIterator) Next() *NeighborEntry {
	if len(i.entries) == 0 {
		return nil
	}
	entry := i.entries[0]
	i.entries = i.entries[1:]
	return entry
}
