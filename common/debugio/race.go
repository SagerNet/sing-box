package debugio

import (
	"net"
	"sync"

	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	N "github.com/sagernet/sing/common/network"
)

type RaceConn struct {
	N.ExtendedConn
	readAccess  sync.Mutex
	writeAccess sync.Mutex
}

func NewRaceConn(conn net.Conn) N.ExtendedConn {
	return &RaceConn{ExtendedConn: bufio.NewExtendedConn(conn)}
}

func (c *RaceConn) Read(p []byte) (n int, err error) {
	c.readAccess.Lock()
	defer c.readAccess.Unlock()
	return c.ExtendedConn.Read(p)
}

func (c *RaceConn) Write(p []byte) (n int, err error) {
	c.writeAccess.Lock()
	defer c.writeAccess.Unlock()
	return c.ExtendedConn.Write(p)
}

func (c *RaceConn) ReadBuffer(buffer *buf.Buffer) error {
	c.readAccess.Lock()
	defer c.readAccess.Unlock()
	return c.ExtendedConn.ReadBuffer(buffer)
}

func (c *RaceConn) WriteBuffer(buffer *buf.Buffer) error {
	c.writeAccess.Lock()
	defer c.writeAccess.Unlock()
	return c.ExtendedConn.WriteBuffer(buffer)
}

func (c *RaceConn) Upstream() any {
	return c.ExtendedConn
}
