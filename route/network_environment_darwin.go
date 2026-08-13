package route

import (
	"net"
	"net/netip"
	"slices"

	"golang.org/x/net/route"
	"golang.org/x/sys/unix"
)

func systemGateways(interfaceIndex int) []netip.Addr {
	rib, err := route.FetchRIB(unix.AF_UNSPEC, route.RIBTypeRoute, 0)
	if err != nil {
		return nil
	}
	messages, err := route.ParseRIB(route.RIBTypeRoute, rib)
	if err != nil {
		return nil
	}
	var gateways []netip.Addr
	for _, message := range messages {
		routeMessage, isRouteMessage := message.(*route.RouteMessage)
		if !isRouteMessage || routeMessage.Index != interfaceIndex || routeMessage.Flags&unix.RTF_GATEWAY == 0 {
			continue
		}
		destination := routeAddressAt(routeMessage.Addrs, unix.RTAX_DST)
		if !destination.IsValid() || !destination.IsUnspecified() {
			continue
		}
		gateway := routeAddressAt(routeMessage.Addrs, unix.RTAX_GATEWAY)
		if gateway.IsValid() {
			gateways = append(gateways, gateway)
		}
	}
	return gateways
}

func systemNeighborHardwareAddresses(interfaceIndex int, addresses []netip.Addr) map[netip.Addr]net.HardwareAddr {
	rib, err := route.FetchRIB(unix.AF_UNSPEC, route.RIBType(unix.NET_RT_FLAGS), unix.RTF_LLINFO)
	if err != nil {
		return nil
	}
	messages, err := route.ParseRIB(route.RIBTypeRoute, rib)
	if err != nil {
		return nil
	}
	hardwareAddresses := make(map[netip.Addr]net.HardwareAddr)
	for _, message := range messages {
		routeMessage, isRouteMessage := message.(*route.RouteMessage)
		if !isRouteMessage || routeMessage.Index != interfaceIndex {
			continue
		}
		destination := routeAddressAt(routeMessage.Addrs, unix.RTAX_DST)
		if !slices.Contains(addresses, destination) {
			continue
		}
		if len(routeMessage.Addrs) <= unix.RTAX_GATEWAY {
			continue
		}
		linkAddress, isLinkAddress := routeMessage.Addrs[unix.RTAX_GATEWAY].(*route.LinkAddr)
		if !isLinkAddress || len(linkAddress.Addr) == 0 {
			continue
		}
		hardwareAddresses[destination] = net.HardwareAddr(linkAddress.Addr)
	}
	return hardwareAddresses
}

func routeAddressAt(addresses []route.Addr, index int) netip.Addr {
	if len(addresses) <= index {
		return netip.Addr{}
	}
	switch address := addresses[index].(type) {
	case *route.Inet4Addr:
		return netip.AddrFrom4(address.IP)
	case *route.Inet6Addr:
		return netip.AddrFrom16(address.IP)
	default:
		return netip.Addr{}
	}
}
