package bridge

import (
	"net"
	"net/netip"
	"os"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/sagernet/sing-tun"
	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/net/route"
	"golang.org/x/sys/unix"
)

var routeMessageSeq atomic.Int32

func interfaceGateway(interfaceIndex int, is4 bool) netip.Addr {
	socketFd, err := unix.Socket(unix.AF_ROUTE, unix.SOCK_RAW, 0)
	if err != nil {
		return netip.Addr{}
	}
	defer unix.Close(socketFd)
	_ = unix.SetsockoptTimeval(socketFd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &unix.Timeval{Sec: 1})
	var destination route.Addr
	if is4 {
		destination = &route.Inet4Addr{}
	} else {
		destination = &route.Inet6Addr{}
	}
	seq := int(routeMessageSeq.Add(1))
	message := route.RouteMessage{
		Type:    unix.RTM_GET,
		Version: unix.RTM_VERSION,
		Flags:   unix.RTF_IFSCOPE,
		Index:   interfaceIndex,
		ID:      uintptr(os.Getpid()),
		Seq:     seq,
		Addrs:   []route.Addr{syscall.RTAX_DST: destination},
	}
	request, err := message.Marshal()
	if err != nil {
		return netip.Addr{}
	}
	_, err = unix.Write(socketFd, request)
	if err != nil {
		return netip.Addr{}
	}
	buffer := make([]byte, 2048)
	for {
		n, err := unix.Read(socketFd, buffer)
		if err != nil {
			return netip.Addr{}
		}
		messages, err := route.ParseRIB(route.RIBTypeRoute, buffer[:n])
		if err != nil {
			continue
		}
		for _, routeMessage := range messages {
			reply, isRoute := routeMessage.(*route.RouteMessage)
			if !isRoute || reply.Seq != seq || reply.ID != uintptr(os.Getpid()) {
				continue
			}
			if reply.Err != nil || reply.Flags&unix.RTF_GATEWAY == 0 || len(reply.Addrs) <= syscall.RTAX_GATEWAY {
				return netip.Addr{}
			}
			switch gateway := reply.Addrs[syscall.RTAX_GATEWAY].(type) {
			case *route.Inet4Addr:
				return netip.AddrFrom4(gateway.IP)
			case *route.Inet6Addr:
				return netip.AddrFrom16(gateway.IP)
			default:
				return netip.Addr{}
			}
		}
	}
}

func addInterfaceHostRoute(destination netip.Addr, interfaceName string) error {
	tunInterface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return err
	}
	var destinationAddr, maskAddr route.Addr
	if destination.Is4() {
		destinationAddr = &route.Inet4Addr{IP: destination.As4()}
		maskAddr = &route.Inet4Addr{IP: [4]byte{255, 255, 255, 255}}
	} else {
		destinationAddr = &route.Inet6Addr{IP: destination.As16()}
		maskAddr = &route.Inet6Addr{IP: [16]byte{
			255, 255, 255, 255, 255, 255, 255, 255,
			255, 255, 255, 255, 255, 255, 255, 255,
		}}
	}
	message := route.RouteMessage{
		Type:    unix.RTM_ADD,
		Version: unix.RTM_VERSION,
		Flags:   unix.RTF_UP | unix.RTF_HOST | unix.RTF_STATIC,
		Seq:     int(routeMessageSeq.Add(1)),
		Addrs: []route.Addr{
			syscall.RTAX_DST:     destinationAddr,
			syscall.RTAX_GATEWAY: &route.LinkAddr{Index: tunInterface.Index},
			syscall.RTAX_NETMASK: maskAddr,
		},
	}
	request, err := message.Marshal()
	if err != nil {
		return err
	}
	socketFd, err := unix.Socket(unix.AF_ROUTE, unix.SOCK_RAW, 0)
	if err != nil {
		return err
	}
	defer unix.Close(socketFd)
	_, err = unix.Write(socketFd, request)
	if err != nil && err != unix.EEXIST {
		return E.Cause(err, "RTM_ADD")
	}
	return nil
}

type ifAliasRequest struct {
	Name    [unix.IFNAMSIZ]byte
	Addr    unix.RawSockaddrInet4
	DstAddr unix.RawSockaddrInet4
	Mask    unix.RawSockaddrInet4
}

type inet6AddrLifetime struct {
	Expire    float64
	Preferred float64
	Vltime    uint32
	Pltime    uint32
}

type ifAliasRequest6 struct {
	Name     [unix.IFNAMSIZ]byte
	Addr     unix.RawSockaddrInet6
	DstAddr  unix.RawSockaddrInet6
	Mask     unix.RawSockaddrInet6
	Flags    uint32
	Lifetime inet6AddrLifetime
}

func assignPointToPointAddress(interfaceName string, local netip.Addr, peer netip.Addr) error {
	if local.Is4() {
		request := ifAliasRequest{
			Addr: unix.RawSockaddrInet4{
				Len:    unix.SizeofSockaddrInet4,
				Family: unix.AF_INET,
				Addr:   local.As4(),
			},
			DstAddr: unix.RawSockaddrInet4{
				Len:    unix.SizeofSockaddrInet4,
				Family: unix.AF_INET,
				Addr:   peer.As4(),
			},
			Mask: unix.RawSockaddrInet4{
				Len:    unix.SizeofSockaddrInet4,
				Family: unix.AF_INET,
				Addr:   [4]byte{255, 255, 255, 255},
			},
		}
		copy(request.Name[:], interfaceName)
		return interfaceIoctl(unix.AF_INET, uint(unix.SIOCAIFADDR), unsafe.Pointer(&request))
	}
	request := ifAliasRequest6{
		Addr: unix.RawSockaddrInet6{
			Len:    unix.SizeofSockaddrInet6,
			Family: unix.AF_INET6,
			Addr:   local.As16(),
		},
		DstAddr: unix.RawSockaddrInet6{
			Len:    unix.SizeofSockaddrInet6,
			Family: unix.AF_INET6,
			Addr:   peer.As16(),
		},
		Mask: unix.RawSockaddrInet6{
			Len:    unix.SizeofSockaddrInet6,
			Family: unix.AF_INET6,
			Addr: [16]byte{
				255, 255, 255, 255, 255, 255, 255, 255,
				255, 255, 255, 255, 255, 255, 255, 255,
			},
		},
		Flags: tun.IN6_IFF_NODAD | tun.IN6_IFF_SECURED,
		Lifetime: inet6AddrLifetime{
			Vltime: tun.ND6_INFINITE_LIFETIME,
			Pltime: tun.ND6_INFINITE_LIFETIME,
		},
	}
	copy(request.Name[:], interfaceName)
	return interfaceIoctl(unix.AF_INET6, tun.SIOCAIFADDR_IN6, unsafe.Pointer(&request))
}

func interfaceIoctl(family int, request uint, pointer unsafe.Pointer) error {
	socketFd, err := unix.Socket(family, unix.SOCK_DGRAM, 0)
	if err != nil {
		return err
	}
	defer unix.Close(socketFd)
	return unixIoctlPtr(socketFd, request, pointer)
}

var forwardingMibs = map[string][]int32{
	// CTL_NET, PF_INET, IPPROTO_IP, IPCTL_FORWARDING (netinet/in.h)
	"net.inet.ip.forwarding": {syscall.CTL_NET, unix.AF_INET, 0, 1},
	// CTL_NET, PF_INET6, IPPROTO_IPV6, IPV6CTL_FORWARDING (netinet6/in6.h)
	"net.inet6.ip6.forwarding": {syscall.CTL_NET, unix.AF_INET6, unix.IPPROTO_IPV6, 1},
}

func getSysctlInt32(mib []int32) (int32, error) {
	var value int32
	valueLen := unsafe.Sizeof(value)
	err := unixSysctl(mib, (*byte)(unsafe.Pointer(&value)), &valueLen, nil, 0)
	return value, err
}

func setSysctlInt32(mib []int32, value int32) error {
	return unixSysctl(mib, nil, nil, (*byte)(unsafe.Pointer(&value)), unsafe.Sizeof(value))
}
