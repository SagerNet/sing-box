package hysteria

import (
	"net"
	"os"
	"syscall"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/sing/common"
)

type PacketConnWrapper struct {
	net.PacketConn
}

func (c *PacketConnWrapper) SetReadBuffer(bytes int) error {
	return common.MustCast[*net.UDPConn](c.PacketConn).SetReadBuffer(bytes)
}

func (c *PacketConnWrapper) SetWriteBuffer(bytes int) error {
	return common.MustCast[*net.UDPConn](c.PacketConn).SetWriteBuffer(bytes)
}

func (c *PacketConnWrapper) SyscallConn() (syscall.RawConn, error) {
	return common.MustCast[*net.UDPConn](c.PacketConn).SyscallConn()
}

func (c *PacketConnWrapper) File() (f *os.File, err error) {
	return common.MustCast[*net.UDPConn](c.PacketConn).File()
}

func (c *PacketConnWrapper) Upstream() any {
	return c.PacketConn
}

type StreamWrapper struct {
	quic.Stream
}

func (s *StreamWrapper) Upstream() any {
	return s.Stream
}

func (s *StreamWrapper) ReaderReplaceable() bool {
	return true
}

func (s *StreamWrapper) WriterReplaceable() bool {
	return true
}

func (s *StreamWrapper) Close() error {
	s.CancelRead(0)
	s.Stream.Close()
	return nil
}
