package conntrack

import (
	"net"
	"runtime/debug"

	"github.com/sagernet/sing/common/x/list"
)

type Conn struct {
	net.Conn
	element *list.Element[*ConnEntry]
}

func NewConn(conn net.Conn) *Conn {
	entry := &ConnEntry{
		Conn:  conn,
		Stack: debug.Stack(),
	}
	connAccess.Lock()
	element := openConnection.PushBack(entry)
	connAccess.Unlock()
	return &Conn{
		Conn:    conn,
		element: element,
	}
}

func (c *Conn) Close() error {
	if c.element.Value != nil {
		connAccess.Lock()
		if c.element.Value != nil {
			openConnection.Remove(c.element)
			c.element.Value = nil
		}
		connAccess.Unlock()
	}
	return c.Conn.Close()
}

func (c *Conn) Upstream() any {
	return c.Conn
}

func (c *Conn) ReaderReplaceable() bool {
	return true
}

func (c *Conn) WriterReplaceable() bool {
	return true
}
