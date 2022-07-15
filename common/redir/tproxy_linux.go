package redir

import (
	"encoding/binary"
	"net"
	"net/netip"
	"os"
	"strconv"
	"syscall"

	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"

	"golang.org/x/sys/unix"
)

func TProxy(fd uintptr, isIPv6 bool) error {
	err := syscall.SetsockoptInt(int(fd), syscall.SOL_IP, syscall.IP_TRANSPARENT, 1)
	if err != nil {
		return err
	}
	if isIPv6 {
		err = syscall.SetsockoptInt(int(fd), syscall.SOL_IPV6, unix.IPV6_TRANSPARENT, 1)
	}
	return err
}

func TProxyUDP(fd uintptr, isIPv6 bool) error {
	err := syscall.SetsockoptInt(int(fd), syscall.SOL_IP, syscall.IP_RECVORIGDSTADDR, 1)
	if err != nil {
		return err
	}
	if isIPv6 {
		err = syscall.SetsockoptInt(int(fd), syscall.SOL_IPV6, unix.IPV6_RECVORIGDSTADDR, 1)
		if err != nil {
			return err
		}
	}
	return nil
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

func DialUDP(lAddr *net.UDPAddr, rAddr *net.UDPAddr) (*net.UDPConn, error) {
	rSockAddr, err := udpAddrToSockAddr(rAddr)
	if err != nil {
		return nil, err
	}

	lSockAddr, err := udpAddrToSockAddr(lAddr)
	if err != nil {
		return nil, err
	}

	fd, err := syscall.Socket(udpAddrFamily(lAddr, rAddr), syscall.SOCK_DGRAM, 0)
	if err != nil {
		return nil, err
	}

	if err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		syscall.Close(fd)
		return nil, err
	}

	if err = syscall.SetsockoptInt(fd, syscall.SOL_IP, syscall.IP_TRANSPARENT, 1); err != nil {
		syscall.Close(fd)
		return nil, err
	}

	if err = syscall.Bind(fd, lSockAddr); err != nil {
		syscall.Close(fd)
		return nil, err
	}

	if err = syscall.Connect(fd, rSockAddr); err != nil {
		syscall.Close(fd)
		return nil, err
	}

	fdFile := os.NewFile(uintptr(fd), F.ToString("net-udp-dial-", rAddr))
	defer fdFile.Close()

	c, err := net.FileConn(fdFile)
	if err != nil {
		syscall.Close(fd)
		return nil, err
	}

	return c.(*net.UDPConn), nil
}

func udpAddrToSockAddr(addr *net.UDPAddr) (syscall.Sockaddr, error) {
	switch {
	case addr.IP.To4() != nil:
		ip := [4]byte{}
		copy(ip[:], addr.IP.To4())

		return &syscall.SockaddrInet4{Addr: ip, Port: addr.Port}, nil

	default:
		ip := [16]byte{}
		copy(ip[:], addr.IP.To16())

		zoneID, err := strconv.ParseUint(addr.Zone, 10, 32)
		if err != nil {
			zoneID = 0
		}

		return &syscall.SockaddrInet6{Addr: ip, Port: addr.Port, ZoneId: uint32(zoneID)}, nil
	}
}

func udpAddrFamily(lAddr, rAddr *net.UDPAddr) int {
	if (lAddr == nil || lAddr.IP.To4() != nil) && (rAddr == nil || lAddr.IP.To4() != nil) {
		return syscall.AF_INET
	}
	return syscall.AF_INET6
}
