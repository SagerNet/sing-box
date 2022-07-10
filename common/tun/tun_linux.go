package tun

import (
	"net"
	"net/netip"

	"github.com/vishvananda/netlink"
	"gvisor.dev/gvisor/pkg/tcpip/link/tun"
)

func Open(name string) (uintptr, error) {
	tunFd, err := tun.Open(name)
	if err != nil {
		return 0, err
	}
	return uintptr(tunFd), nil
}

func Configure(name string, inet4Address netip.Prefix, inet6Address netip.Prefix, mtu uint32, autoRoute bool) error {
	tunLink, err := netlink.LinkByName(name)
	if err != nil {
		return err
	}

	if inet4Address.IsValid() {
		addr4, _ := netlink.ParseAddr(inet4Address.String())
		err = netlink.AddrAdd(tunLink, addr4)
		if err != nil {
			return err
		}
	}

	if inet6Address.IsValid() {
		addr6, _ := netlink.ParseAddr(inet6Address.String())
		err = netlink.AddrAdd(tunLink, addr6)
		if err != nil {
			return err
		}
	}

	err = netlink.LinkSetMTU(tunLink, int(mtu))
	if err != nil {
		return err
	}

	err = netlink.LinkSetUp(tunLink)
	if err != nil {
		return err
	}

	if autoRoute {
		if inet4Address.IsValid() {
			err = netlink.RouteAdd(&netlink.Route{
				Dst: &net.IPNet{
					IP:   net.IPv4zero,
					Mask: net.CIDRMask(0, 32),
				},
				LinkIndex: tunLink.Attrs().Index,
			})
			if err != nil {
				return err
			}
		}
		if inet6Address.IsValid() {
			err = netlink.RouteAdd(&netlink.Route{
				Dst: &net.IPNet{
					IP:   net.IPv6zero,
					Mask: net.CIDRMask(0, 128),
				},
				LinkIndex: tunLink.Attrs().Index,
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func UnConfigure(name string, inet4Address netip.Prefix, inet6Address netip.Prefix, autoRoute bool) error {
	if autoRoute {
		tunLink, err := netlink.LinkByName(name)
		if err != nil {
			return err
		}
		if inet4Address.IsValid() {
			err = netlink.RouteDel(&netlink.Route{
				Dst: &net.IPNet{
					IP:   net.IPv4zero,
					Mask: net.CIDRMask(0, 32),
				},
				LinkIndex: tunLink.Attrs().Index,
			})
			if err != nil {
				return err
			}
		}
		if inet6Address.IsValid() {
			err = netlink.RouteDel(&netlink.Route{
				Dst: &net.IPNet{
					IP:   net.IPv6zero,
					Mask: net.CIDRMask(0, 128),
				},
				LinkIndex: tunLink.Attrs().Index,
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}
