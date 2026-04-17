//go:build linux || darwin

package tlsspoof

import (
	"net"
	"net/netip"

	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/unix"
)

func openIPv4RawSocket(dst netip.AddrPort) (int, unix.Sockaddr, error) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_RAW, unix.IPPROTO_TCP)
	if err != nil {
		return -1, nil, E.Cause(err, "open AF_INET SOCK_RAW")
	}
	err = unix.SetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_HDRINCL, 1)
	if err != nil {
		unix.Close(fd)
		return -1, nil, E.Cause(err, "set IP_HDRINCL")
	}
	sockaddr := &unix.SockaddrInet4{Port: int(dst.Port())}
	sockaddr.Addr = dst.Addr().As4()
	return fd, sockaddr, nil
}

// readTCPMaxSeg reads the negotiated MSS from the TCP connection. Called after
// the TCP handshake has completed, so the value reflects what the peer
// advertised rather than the kernel's default.
func readTCPMaxSeg(tcpConn *net.TCPConn) (int, error) {
	var mss int
	err := control.Conn(tcpConn, func(raw uintptr) error {
		value, getErr := unix.GetsockoptInt(int(raw), unix.IPPROTO_TCP, unix.TCP_MAXSEG)
		if getErr != nil {
			return getErr
		}
		mss = value
		return nil
	})
	if err != nil {
		return 0, err
	}
	return mss, nil
}
