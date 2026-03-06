//go:build darwin

package route

import (
	"net"
	"net/netip"
	"syscall"

	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/net/route"
	"golang.org/x/sys/unix"
)

func ReadNeighborEntries() ([]adapter.NeighborEntry, error) {
	var entries []adapter.NeighborEntry
	ipv4Entries, err := readNeighborEntriesAF(syscall.AF_INET)
	if err != nil {
		return nil, E.Cause(err, "read IPv4 neighbors")
	}
	entries = append(entries, ipv4Entries...)
	ipv6Entries, err := readNeighborEntriesAF(syscall.AF_INET6)
	if err != nil {
		return nil, E.Cause(err, "read IPv6 neighbors")
	}
	entries = append(entries, ipv6Entries...)
	return entries, nil
}

func readNeighborEntriesAF(addressFamily int) ([]adapter.NeighborEntry, error) {
	rib, err := route.FetchRIB(addressFamily, route.RIBType(syscall.NET_RT_FLAGS), syscall.RTF_LLINFO)
	if err != nil {
		return nil, err
	}
	messages, err := route.ParseRIB(route.RIBType(syscall.NET_RT_FLAGS), rib)
	if err != nil {
		return nil, err
	}
	var entries []adapter.NeighborEntry
	for _, message := range messages {
		routeMessage, isRouteMessage := message.(*route.RouteMessage)
		if !isRouteMessage {
			continue
		}
		address, macAddress, ok := parseRouteNeighborEntry(routeMessage)
		if !ok {
			continue
		}
		entries = append(entries, adapter.NeighborEntry{
			Address:    address,
			MACAddress: macAddress,
		})
	}
	return entries, nil
}

func parseRouteNeighborEntry(message *route.RouteMessage) (address netip.Addr, macAddress net.HardwareAddr, ok bool) {
	if len(message.Addrs) <= unix.RTAX_GATEWAY {
		return
	}
	gateway, isLinkAddr := message.Addrs[unix.RTAX_GATEWAY].(*route.LinkAddr)
	if !isLinkAddr || len(gateway.Addr) < 6 {
		return
	}
	switch destination := message.Addrs[unix.RTAX_DST].(type) {
	case *route.Inet4Addr:
		address = netip.AddrFrom4(destination.IP)
	case *route.Inet6Addr:
		address = netip.AddrFrom16(destination.IP)
	default:
		return
	}
	macAddress = net.HardwareAddr(make([]byte, len(gateway.Addr)))
	copy(macAddress, gateway.Addr)
	ok = true
	return
}

func ParseRouteNeighborMessage(message *route.RouteMessage) (address netip.Addr, macAddress net.HardwareAddr, isDelete bool, ok bool) {
	isDelete = message.Type == unix.RTM_DELETE
	if len(message.Addrs) <= unix.RTAX_GATEWAY {
		return
	}
	switch destination := message.Addrs[unix.RTAX_DST].(type) {
	case *route.Inet4Addr:
		address = netip.AddrFrom4(destination.IP)
	case *route.Inet6Addr:
		address = netip.AddrFrom16(destination.IP)
	default:
		return
	}
	if !isDelete {
		gateway, isLinkAddr := message.Addrs[unix.RTAX_GATEWAY].(*route.LinkAddr)
		if !isLinkAddr || len(gateway.Addr) < 6 {
			return
		}
		macAddress = net.HardwareAddr(make([]byte, len(gateway.Addr)))
		copy(macAddress, gateway.Addr)
	}
	ok = true
	return
}
