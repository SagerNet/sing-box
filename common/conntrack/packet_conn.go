package conntrack

import (
	"io"
	"net"

	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/x/list"
)

type PacketConn struct {
	net.PacketConn
	element *list.Element[io.Closer]
}

func NewPacketConn(conn net.PacketConn) (net.PacketConn, error) {
	connAccess.Lock()
	element := openConnection.PushBack(conn)
	connAccess.Unlock()
	if KillerEnabled {
		err := KillerCheck()
		if err != nil {
			conn.Close()
			return nil, err
		}
	}
	return &PacketConn{
		PacketConn: conn,
		element:    element,
	}, nil
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
	return bufio.NewPacketConn(c.PacketConn)
}

func (c *PacketConn) ReaderReplaceable() bool {
	return true
}

func (c *PacketConn) WriterReplaceable() bool {
	return true
}
