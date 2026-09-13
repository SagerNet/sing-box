package interrupt

import (
	"net"

	"github.com/sagernet/sing/common/bufio"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/x/list"
)

type Conn struct {
	net.Conn
	group   *Group
	element *list.Element[*groupConnItem]
}

func (c *Conn) Close() error {
	c.group.access.Lock()
	defer c.group.access.Unlock()
	c.group.connections.Remove(c.element)
	return c.Conn.Close()
}

func (c *Conn) ReaderReplaceable() bool {
	return true
}

func (c *Conn) WriterReplaceable() bool {
	return true
}

func (c *Conn) Upstream() any {
	return c.Conn
}

type PacketConn struct {
	N.NetPacketConn
	group   *Group
	element *list.Element[*groupConnItem]
}

func newPacketConn(group *Group, conn net.PacketConn, element *list.Element[*groupConnItem]) *PacketConn {
	return &PacketConn{NetPacketConn: bufio.NewPacketConn(conn), group: group, element: element}
}

func (c *PacketConn) Close() error {
	c.group.access.Lock()
	defer c.group.access.Unlock()
	c.group.connections.Remove(c.element)
	return c.NetPacketConn.Close()
}

func (c *PacketConn) ReaderReplaceable() bool {
	return true
}

func (c *PacketConn) WriterReplaceable() bool {
	return true
}

func (c *PacketConn) Upstream() any {
	return c.NetPacketConn
}
