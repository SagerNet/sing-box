package conntrack

import (
	"net"
	"runtime/debug"

	"github.com/sagernet/sing/common/x/list"
)

type PacketConn struct {
	net.PacketConn
	element *list.Element[*ConnEntry]
}

func NewPacketConn(conn net.PacketConn) *PacketConn {
	entry := &ConnEntry{
		Conn:  conn,
		Stack: debug.Stack(),
	}
	connAccess.Lock()
	element := openConnection.PushBack(entry)
	connAccess.Unlock()
	return &PacketConn{
		PacketConn: conn,
		element:    element,
	}
}

func (c *PacketConn) Close() error {
	if c.element.Value != nil {
		connAccess.Lock()
		if c.element.Value != nil {
			openConnection.Remove(c.element)
			c.element.Value = nil
		}
		connAccess.Unlock()
	}
	return c.PacketConn.Close()
}

func (c *PacketConn) Upstream() any {
	return c.PacketConn
}

func (c *PacketConn) ReaderReplaceable() bool {
	return true
}

func (c *PacketConn) WriterReplaceable() bool {
	return true
}
