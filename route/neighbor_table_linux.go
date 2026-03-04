//go:build linux

package route

import (
	"net"
	"net/netip"
	"slices"

	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/jsimonetti/rtnetlink"
	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
)

func ReadNeighborEntries() ([]adapter.NeighborEntry, error) {
	connection, err := rtnetlink.Dial(nil)
	if err != nil {
		return nil, E.Cause(err, "dial rtnetlink")
	}
	defer connection.Close()
	neighbors, err := connection.Neigh.List()
	if err != nil {
		return nil, E.Cause(err, "list neighbors")
	}
	var entries []adapter.NeighborEntry
	for _, neighbor := range neighbors {
		if neighbor.Attributes == nil {
			continue
		}
		if neighbor.Attributes.LLAddress == nil || len(neighbor.Attributes.Address) == 0 {
			continue
		}
		address, ok := netip.AddrFromSlice(neighbor.Attributes.Address)
		if !ok {
			continue
		}
		entries = append(entries, adapter.NeighborEntry{
			Address:    address,
			MACAddress: slices.Clone(neighbor.Attributes.LLAddress),
		})
	}
	return entries, nil
}

func ParseNeighborMessage(message netlink.Message) (address netip.Addr, macAddress net.HardwareAddr, isDelete bool, ok bool) {
	var neighMessage rtnetlink.NeighMessage
	err := neighMessage.UnmarshalBinary(message.Data)
	if err != nil {
		return
	}
	if neighMessage.Attributes == nil || len(neighMessage.Attributes.Address) == 0 {
		return
	}
	address, ok = netip.AddrFromSlice(neighMessage.Attributes.Address)
	if !ok {
		return
	}
	isDelete = message.Header.Type == unix.RTM_DELNEIGH
	if !isDelete && neighMessage.Attributes.LLAddress == nil {
		ok = false
		return
	}
	macAddress = slices.Clone(neighMessage.Attributes.LLAddress)
	return
}
