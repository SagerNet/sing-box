package hysteria

import (
	"net"
	"os"
	"syscall"

	"github.com/sagernet/sing/common"
)

type WrapPacketConn struct {
	net.PacketConn
}

func (c *WrapPacketConn) SetReadBuffer(bytes int) error {
	return common.MustCast[*net.UDPConn](c.PacketConn).SetReadBuffer(bytes)
}

func (c *WrapPacketConn) SetWriteBuffer(bytes int) error {
	return common.MustCast[*net.UDPConn](c.PacketConn).SetWriteBuffer(bytes)
}

func (c *WrapPacketConn) SyscallConn() (syscall.RawConn, error) {
	return common.MustCast[*net.UDPConn](c.PacketConn).SyscallConn()
}

func (c *WrapPacketConn) File() (f *os.File, err error) {
	return common.MustCast[*net.UDPConn](c.PacketConn).File()
}
