//go:build linux

package libbox

import (
	"net"
	"net/netip"
	"slices"
	"time"

	"github.com/sagernet/sing-box/route"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
)

type NeighborEntry struct {
	Address    string
	MACAddress string
	Hostname   string
}

type NeighborEntryIterator interface {
	Next() *NeighborEntry
	HasNext() bool
}

type NeighborSubscription struct {
	done chan struct{}
}

func SubscribeNeighborTable(listener NeighborUpdateListener) (*NeighborSubscription, error) {
	entries, err := route.ReadNeighborEntries()
	if err != nil {
		return nil, E.Cause(err, "initial neighbor dump")
	}
	table := make(map[netip.Addr]net.HardwareAddr)
	for _, entry := range entries {
		table[entry.Address] = entry.MACAddress
	}
	listener.UpdateNeighborTable(tableToIterator(table))
	connection, err := netlink.Dial(unix.NETLINK_ROUTE, &netlink.Config{
		Groups: 1 << (unix.RTNLGRP_NEIGH - 1),
	})
	if err != nil {
		return nil, E.Cause(err, "subscribe neighbor updates")
	}
	subscription := &NeighborSubscription{
		done: make(chan struct{}),
	}
	go subscription.loop(listener, connection, table)
	return subscription, nil
}

func (s *NeighborSubscription) Close() {
	close(s.done)
}

func (s *NeighborSubscription) loop(listener NeighborUpdateListener, connection *netlink.Conn, table map[netip.Addr]net.HardwareAddr) {
	defer connection.Close()
	for {
		select {
		case <-s.done:
			return
		default:
		}
		err := connection.SetReadDeadline(time.Now().Add(3 * time.Second))
		if err != nil {
			return
		}
		messages, err := connection.Receive()
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
				continue
			}
			select {
			case <-s.done:
				return
			default:
			}
			continue
		}
		changed := false
		for _, message := range messages {
			address, mac, isDelete, ok := route.ParseNeighborMessage(message)
			if !ok {
				continue
			}
			if isDelete {
				if _, exists := table[address]; exists {
					delete(table, address)
					changed = true
				}
			} else {
				existing, exists := table[address]
				if !exists || !slices.Equal(existing, mac) {
					table[address] = mac
					changed = true
				}
			}
		}
		if changed {
			listener.UpdateNeighborTable(tableToIterator(table))
		}
	}
}

func tableToIterator(table map[netip.Addr]net.HardwareAddr) NeighborEntryIterator {
	entries := make([]*NeighborEntry, 0, len(table))
	for address, mac := range table {
		entries = append(entries, &NeighborEntry{
			Address:    address.String(),
			MACAddress: mac.String(),
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
