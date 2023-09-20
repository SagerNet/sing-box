package conntrack

import (
	"io"
	"net"

	"github.com/sagernet/sing/common/x/list"
)

type Conn struct {
	net.Conn
	element *list.Element[io.Closer]
}

func NewConn(conn net.Conn) (net.Conn, error) {
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
	return &Conn{
		Conn:    conn,
		element: element,
	}, nil
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
