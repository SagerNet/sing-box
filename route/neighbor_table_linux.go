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

func SubscribeNeighborTable() (*netlink.Conn, map[netip.Addr]net.HardwareAddr, error) {
	connection, err := netlink.Dial(unix.NETLINK_ROUTE, &netlink.Config{
		Groups: 1 << (unix.RTNLGRP_NEIGH - 1),
	})
	if err != nil {
		return nil, nil, E.Cause(err, "subscribe neighbor updates")
	}
	entries, err := ReadNeighborEntries()
	if err != nil {
		connection.Close()
		return nil, nil, E.Cause(err, "load neighbor table")
	}
	table := make(map[netip.Addr]net.HardwareAddr, len(entries))
	for _, entry := range entries {
		table[entry.Address] = entry.MACAddress
	}
	return connection, table, nil
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
