package redir

import (
	"encoding/binary"
	"net"
	"net/netip"
	"os"
	"syscall"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/control"
	M "github.com/sagernet/sing/common/metadata"
)

func GetOriginalDestination(conn net.Conn) (destination netip.AddrPort, err error) {
	syscallConn, ok := common.Cast[syscall.Conn](conn)
	if !ok {
		return netip.AddrPort{}, os.ErrInvalid
	}
	err = control.Conn(syscallConn, func(fd uintptr) error {
		const SO_ORIGINAL_DST = 80
		if conn.RemoteAddr().(*net.TCPAddr).IP.To4() != nil {
			raw, err := syscall.GetsockoptIPv6Mreq(int(fd), syscall.IPPROTO_IP, SO_ORIGINAL_DST)
			if err != nil {
				return err
			}
			destination = netip.AddrPortFrom(M.AddrFromIP(raw.Multiaddr[4:8]), uint16(raw.Multiaddr[2])<<8+uint16(raw.Multiaddr[3]))
		} else {
			raw, err := syscall.GetsockoptIPv6MTUInfo(int(fd), syscall.IPPROTO_IPV6, SO_ORIGINAL_DST)
			if err != nil {
				return err
			}
			var port [2]byte
			binary.BigEndian.PutUint16(port[:], raw.Addr.Port)
			destination = netip.AddrPortFrom(M.AddrFromIP(raw.Addr.Addr[:]), binary.LittleEndian.Uint16(port[:]))
		}
		return nil
	})
	return
}
