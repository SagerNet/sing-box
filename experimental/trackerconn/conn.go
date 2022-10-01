package trackerconn

import (
	"io"
	"net"

	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	N "github.com/sagernet/sing/common/network"

	"go.uber.org/atomic"
)

func New(conn net.Conn, readCounter *atomic.Int64, writeCounter *atomic.Int64, direct bool) N.ExtendedConn {
	trackerConn := &Conn{bufio.NewExtendedConn(conn), readCounter, writeCounter}
	if direct {
		return (*DirectConn)(trackerConn)
	} else {
		return trackerConn
	}
}

func NewHook(conn net.Conn, readCounter func(n int64), writeCounter func(n int64), direct bool) N.ExtendedConn {
	trackerConn := &HookConn{bufio.NewExtendedConn(conn), readCounter, writeCounter}
	if direct {
		return (*DirectHookConn)(trackerConn)
	} else {
		return trackerConn
	}
}

type Conn struct {
	N.ExtendedConn
	readCounter  *atomic.Int64
	writeCounter *atomic.Int64
}

func (c *Conn) Read(p []byte) (n int, err error) {
	n, err = c.ExtendedConn.Read(p)
	c.readCounter.Add(int64(n))
	return n, err
}

func (c *Conn) ReadBuffer(buffer *buf.Buffer) error {
	err := c.ExtendedConn.ReadBuffer(buffer)
	if err != nil {
		return err
	}
	c.readCounter.Add(int64(buffer.Len()))
	return nil
}

func (c *Conn) Write(p []byte) (n int, err error) {
	n, err = c.ExtendedConn.Write(p)
	c.writeCounter.Add(int64(n))
	return n, err
}

func (c *Conn) WriteBuffer(buffer *buf.Buffer) error {
	dataLen := int64(buffer.Len())
	err := c.ExtendedConn.WriteBuffer(buffer)
	if err != nil {
		return err
	}
	c.writeCounter.Add(dataLen)
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

type DirectConn Conn

func (c *DirectConn) WriteTo(w io.Writer) (n int64, err error) {
	reader := N.UnwrapReader(c.ExtendedConn)
	if wt, ok := reader.(io.WriterTo); ok {
		n, err = wt.WriteTo(w)
		c.readCounter.Add(n)
		return
	} else {
		return bufio.Copy(w, (*Conn)(c))
	}
}

func (c *DirectConn) ReadFrom(r io.Reader) (n int64, err error) {
	writer := N.UnwrapWriter(c.ExtendedConn)
	if rt, ok := writer.(io.ReaderFrom); ok {
		n, err = rt.ReadFrom(r)
		c.writeCounter.Add(n)
		return
	} else {
		return bufio.Copy((*Conn)(c), r)
	}
}

type DirectHookConn HookConn

func (c *DirectHookConn) WriteTo(w io.Writer) (n int64, err error) {
	reader := N.UnwrapReader(c.ExtendedConn)
	if wt, ok := reader.(io.WriterTo); ok {
		n, err = wt.WriteTo(w)
		c.readCounter(n)
		return
	} else {
		return bufio.Copy(w, (*HookConn)(c))
	}
}

func (c *DirectHookConn) ReadFrom(r io.Reader) (n int64, err error) {
	writer := N.UnwrapWriter(c.ExtendedConn)
	if rt, ok := writer.(io.ReaderFrom); ok {
		n, err = rt.ReadFrom(r)
		c.writeCounter(n)
		return
	} else {
		return bufio.Copy((*HookConn)(c), r)
	}
}
