package openvpn

import (
	"syscall"

	"github.com/sagernet/sing/common"

	"golang.org/x/sys/unix"
)

const openVPNUDPSocketBufferSize = 7 << 20

func tuneOpenVPNUDPSocket(connection any) {
	syscallConnection, loaded := common.Cast[syscall.Conn](connection)
	if !loaded {
		return
	}
	rawConnection, err := syscallConnection.SyscallConn()
	if err != nil {
		return
	}
	_ = rawConnection.Control(func(fd uintptr) {
		_ = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_RCVBUF, openVPNUDPSocketBufferSize)
		_ = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_SNDBUF, openVPNUDPSocketBufferSize)
		_ = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_RCVBUFFORCE, openVPNUDPSocketBufferSize)
		_ = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_SNDBUFFORCE, openVPNUDPSocketBufferSize)
	})
}
