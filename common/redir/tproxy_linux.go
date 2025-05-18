package redir

import (
	"encoding/binary"
	"net/netip"
	"syscall"

	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"

	"golang.org/x/sys/unix"
)

func TProxy(fd uintptr, isIPv6 bool, isUDP bool) error {
	err := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	if err == nil {
		err = syscall.SetsockoptInt(int(fd), syscall.SOL_IP, syscall.IP_TRANSPARENT, 1)
	}
	if err == nil && isIPv6 {
		err = syscall.SetsockoptInt(int(fd), syscall.SOL_IPV6, unix.IPV6_TRANSPARENT, 1)
	}
	if isUDP {
		if err == nil {
			err = syscall.SetsockoptInt(int(fd), syscall.SOL_IP, syscall.IP_RECVORIGDSTADDR, 1)
		}
		if err == nil && isIPv6 {
			err = syscall.SetsockoptInt(int(fd), syscall.SOL_IPV6, unix.IPV6_RECVORIGDSTADDR, 1)
		}
	}
	return err
}

func TProxyWriteBack() control.Func {
	return func(network, address string, conn syscall.RawConn) error {
		return control.Raw(conn, func(fd uintptr) error {
			if M.ParseSocksaddr(address).Addr.Is6() {
				return syscall.SetsockoptInt(int(fd), syscall.SOL_IPV6, unix.IPV6_TRANSPARENT, 1)
			} else {
				return syscall.SetsockoptInt(int(fd), syscall.SOL_IP, syscall.IP_TRANSPARENT, 1)
			}
		})
	}
}

func GetOriginalDestinationFromOOB(oob []byte) (netip.AddrPort, error) {
	controlMessages, err := unix.ParseSocketControlMessage(oob)
	if err != nil {
		return netip.AddrPort{}, err
	}
	for _, message := range controlMessages {
		if message.Header.Level == unix.SOL_IP && message.Header.Type == unix.IP_RECVORIGDSTADDR {
			return netip.AddrPortFrom(M.AddrFromIP(message.Data[4:8]), binary.BigEndian.Uint16(message.Data[2:4])), nil
		} else if message.Header.Level == unix.SOL_IPV6 && message.Header.Type == unix.IPV6_RECVORIGDSTADDR {
			return netip.AddrPortFrom(M.AddrFromIP(message.Data[8:24]), binary.BigEndian.Uint16(message.Data[2:4])), nil
		}
	}
	return netip.AddrPort{}, E.New("not found")
}
