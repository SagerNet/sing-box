package trackerconn

import (
	"net"

	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	N "github.com/sagernet/sing/common/network"

	"go.uber.org/atomic"
)

func New(conn net.Conn, readCounter []*atomic.Int64, writeCounter []*atomic.Int64, direct bool) *Conn {
	return &Conn{bufio.NewExtendedConn(conn), readCounter, writeCounter}
}

func NewHook(conn net.Conn, readCounter func(n int64), writeCounter func(n int64), direct bool) *HookConn {
	return &HookConn{bufio.NewExtendedConn(conn), readCounter, writeCounter}
}

type Conn struct {
	N.ExtendedConn
	readCounter  []*atomic.Int64
	writeCounter []*atomic.Int64
}

func (c *Conn) Read(p []byte) (n int, err error) {
	n, err = c.ExtendedConn.Read(p)
	for _, counter := range c.readCounter {
		counter.Add(int64(n))
	}
	return n, err
}

func (c *Conn) ReadBuffer(buffer *buf.Buffer) error {
	err := c.ExtendedConn.ReadBuffer(buffer)
	if err != nil {
		return err
	}
	for _, counter := range c.readCounter {
		counter.Add(int64(buffer.Len()))
	}
	return nil
}

func (c *Conn) Write(p []byte) (n int, err error) {
	n, err = c.ExtendedConn.Write(p)
	for _, counter := range c.writeCounter {
		counter.Add(int64(n))
	}
	return n, err
}

func (c *Conn) WriteBuffer(buffer *buf.Buffer) error {
	dataLen := int64(buffer.Len())
	err := c.ExtendedConn.WriteBuffer(buffer)
	if err != nil {
		return err
	}
	for _, counter := range c.writeCounter {
		counter.Add(dataLen)
	}
	return nil
}

func (c *Conn) Upstream() any {
	return c.ExtendedConn
}

type HookConn struct {
	N.ExtendedConn
	readCounter  func(n int64)
	writeCounter func(n int64)
}

func (c *HookConn) Read(p []byte) (n int, err error) {
	n, err = c.ExtendedConn.Read(p)
	c.readCounter(int64(n))
	return n, err
}

func (c *HookConn) ReadBuffer(buffer *buf.Buffer) error {
	err := c.ExtendedConn.ReadBuffer(buffer)
	if err != nil {
		return err
	}
	c.readCounter(int64(buffer.Len()))
	return nil
}

func (c *HookConn) Write(p []byte) (n int, err error) {
	n, err = c.ExtendedConn.Write(p)
	c.writeCounter(int64(n))
	return n, err
}

func (c *HookConn) WriteBuffer(buffer *buf.Buffer) error {
	dataLen := int64(buffer.Len())
	err := c.ExtendedConn.WriteBuffer(buffer)
	if err != nil {
		return err
	}
	c.writeCounter(dataLen)
	return nil
}

func (c *HookConn) Upstream() any {
	return c.ExtendedConn
}
