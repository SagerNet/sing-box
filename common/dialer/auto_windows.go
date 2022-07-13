package dialer

import (
	"encoding/binary"
	"net"
	"net/netip"
	"syscall"
	"unsafe"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/windows"
)

const (
	IP_UNICAST_IF   = 31
	IPV6_UNICAST_IF = 31
)

func bind4(handle windows.Handle, ifaceIdx int) error {
	var bytes [4]byte
	binary.BigEndian.PutUint32(bytes[:], uint32(ifaceIdx))
	idx := *(*uint32)(unsafe.Pointer(&bytes[0]))
	return windows.SetsockoptInt(handle, windows.IPPROTO_IP, IP_UNICAST_IF, int(idx))
}

func bind6(handle windows.Handle, ifaceIdx int) error {
	return windows.SetsockoptInt(handle, windows.IPPROTO_IPV6, IPV6_UNICAST_IF, int(ifaceIdx))
}

func BindToInterface(router adapter.Router) control.Func {
	return func(network, address string, conn syscall.RawConn) error {
		interfaceName := router.DefaultInterfaceName()
		if interfaceName == "" {
			return nil
		}
		ipStr, _, err := net.SplitHostPort(address)
		if err == nil {
			if ip, err := netip.ParseAddr(ipStr); err == nil && !ip.IsGlobalUnicast() {
				return err
			}
		}
		var innerErr error
		err = conn.Control(func(fd uintptr) {
			handle := windows.Handle(fd)
			// handle ip empty, e.g. net.Listen("udp", ":0")
			if ipStr == "" {
				innerErr = bind4(handle, router.DefaultInterfaceIndex())
				if innerErr != nil {
					return
				}
				// try bind ipv6, if failed, ignore. it's a workaround for windows disable interface ipv6
				bind6(handle, router.DefaultInterfaceIndex())
				return
			}

			switch network {
			case "tcp4", "udp4", "ip4":
				innerErr = bind4(handle, router.DefaultInterfaceIndex())
			case "tcp6", "udp6":
				innerErr = bind6(handle, router.DefaultInterfaceIndex())
			}
		})
		return E.Errors(innerErr, err)
	}
}
