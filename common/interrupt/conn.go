package interrupt

import (
	"net"

	"github.com/sagernet/sing/common/x/list"
)

/*type GroupedConn interface {
	MarkAsInternal()
}

func MarkAsInternal(conn any) {
	if groupedConn, isGroupConn := common.Cast[GroupedConn](conn); isGroupConn {
		groupedConn.MarkAsInternal()
	}
}*/

type Conn struct {
	net.Conn
	group   *Group
	element *list.Element[*groupConnItem]
}

/*func (c *Conn) MarkAsInternal() {
	c.element.Value.internal = true
}*/

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
	net.PacketConn
	group   *Group
	element *list.Element[*groupConnItem]
}

/*func (c *PacketConn) MarkAsInternal() {
	c.element.Value.internal = true
}*/

func (c *PacketConn) Close() error {
	c.group.access.Lock()
	defer c.group.access.Unlock()
	c.group.connections.Remove(c.element)
	return c.PacketConn.Close()
}

func (c *PacketConn) ReaderReplaceable() bool {
	return true
}

func (c *PacketConn) WriterReplaceable() bool {
	return true
}

func (c *PacketConn) Upstream() any {
	return c.PacketConn
}
