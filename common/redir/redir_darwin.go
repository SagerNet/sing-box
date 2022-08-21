package redir

import (
	"net"
	"net/netip"
	"syscall"
	"unsafe"

	M "github.com/sagernet/sing/common/metadata"
)

const (
	PF_OUT      = 0x2
	DIOCNATLOOK = 0xc0544417
)

func GetOriginalDestination(conn net.Conn) (destination netip.AddrPort, err error) {
	fd, err := syscall.Open("/dev/pf", 0, syscall.O_RDONLY)
	if err != nil {
		return netip.AddrPort{}, err
	}
	defer syscall.Close(fd)
	nl := struct {
		saddr, daddr, rsaddr, rdaddr       [16]byte
		sxport, dxport, rsxport, rdxport   [4]byte
		af, proto, protoVariant, direction uint8
	}{
		af:        syscall.AF_INET,
		proto:     syscall.IPPROTO_TCP,
		direction: PF_OUT,
	}
	la := conn.LocalAddr().(*net.TCPAddr)
	ra := conn.RemoteAddr().(*net.TCPAddr)
	raIP, laIP := ra.IP, la.IP
	raPort, laPort := ra.Port, la.Port
	switch {
	case raIP.To4() != nil:
		copy(nl.saddr[:net.IPv4len], raIP.To4())
		copy(nl.daddr[:net.IPv4len], laIP.To4())
		nl.af = syscall.AF_INET
	default:
		copy(nl.saddr[:], raIP.To16())
		copy(nl.daddr[:], laIP.To16())
		nl.af = syscall.AF_INET6
	}
	nl.sxport[0], nl.sxport[1] = byte(raPort>>8), byte(raPort)
	nl.dxport[0], nl.dxport[1] = byte(laPort>>8), byte(laPort)
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), DIOCNATLOOK, uintptr(unsafe.Pointer(&nl))); errno != 0 {
		return netip.AddrPort{}, errno
	}

	var ip net.IP
	switch nl.af {
	case syscall.AF_INET:
		ip = make(net.IP, net.IPv4len)
		copy(ip, nl.rdaddr[:net.IPv4len])
	case syscall.AF_INET6:
		ip = make(net.IP, net.IPv6len)
		copy(ip, nl.rdaddr[:])
	}
	port := uint16(nl.rdxport[0])<<8 | uint16(nl.rdxport[1])
	destination = netip.AddrPortFrom(M.AddrFromIP(ip), port)
	return
}
