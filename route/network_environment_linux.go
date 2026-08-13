package route

import (
	"net"
	"net/netip"
	"slices"

	"github.com/sagernet/netlink"
)

func systemGateways(interfaceIndex int) []netip.Addr {
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_ALL, &netlink.Route{LinkIndex: interfaceIndex}, netlink.RT_FILTER_OIF)
	if err != nil {
		return nil
	}
	var gateways []netip.Addr
	for _, currentRoute := range routes {
		if currentRoute.Gw == nil {
			continue
		}
		if currentRoute.Dst != nil {
			ones, _ := currentRoute.Dst.Mask.Size()
			if ones != 0 {
				continue
			}
		}
		gateway, valid := netip.AddrFromSlice(currentRoute.Gw)
		if valid {
			gateways = append(gateways, gateway.Unmap())
		}
	}
	return gateways
}

func systemNeighborHardwareAddresses(interfaceIndex int, addresses []netip.Addr) map[netip.Addr]net.HardwareAddr {
	neighbors, err := netlink.NeighList(interfaceIndex, netlink.FAMILY_ALL)
	if err != nil {
		return nil
	}
	hardwareAddresses := make(map[netip.Addr]net.HardwareAddr)
	for _, neighbor := range neighbors {
		if neighbor.State&(netlink.NUD_INCOMPLETE|netlink.NUD_FAILED) != 0 {
			continue
		}
		if len(neighbor.HardwareAddr) == 0 {
			continue
		}
		neighborAddress, valid := netip.AddrFromSlice(neighbor.IP)
		if !valid {
			continue
		}
		neighborAddress = neighborAddress.Unmap()
		if !slices.Contains(addresses, neighborAddress) {
			continue
		}
		hardwareAddresses[neighborAddress] = neighbor.HardwareAddr
	}
	return hardwareAddresses
}
