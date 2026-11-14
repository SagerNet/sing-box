//go:build linux

package libbox

import (
	"errors"
	"net"
	"net/netip"
	"slices"
	"time"

	"github.com/sagernet/sing-box/route"

	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
)

func SubscribeNeighborTable(listener NeighborUpdateListener) (*NeighborSubscription, error) {
	connection, table, err := route.SubscribeNeighborTable()
	if err != nil {
		return nil, err
	}
	listener.UpdateNeighborTable(tableToIterator(table))
	subscription := &NeighborSubscription{
		done: make(chan struct{}),
	}
	go subscription.loop(listener, connection, table)
	return subscription, nil
}

func (s *NeighborSubscription) loop(listener NeighborUpdateListener, connection *netlink.Conn, table map[netip.Addr]net.HardwareAddr) {
	defer func() {
		connection.Close()
	}()
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
			if errors.Is(err, unix.ENOBUFS) {
				newConnection, newTable, err := route.SubscribeNeighborTable()
				if err != nil {
					continue
				}
				connection.Close()
				connection, table = newConnection, newTable
				listener.UpdateNeighborTable(tableToIterator(table))
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
