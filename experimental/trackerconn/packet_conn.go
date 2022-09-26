package trackerconn

import (
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"go.uber.org/atomic"
)

type PacketConn struct {
	N.PacketConn
	readCounter  *atomic.Int64
	writeCounter *atomic.Int64
}

func NewPacket(conn N.PacketConn, readCounter *atomic.Int64, writeCounter *atomic.Int64) *PacketConn {
	return &PacketConn{conn, readCounter, writeCounter}
}

func (c *PacketConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	destination, err = c.PacketConn.ReadPacket(buffer)
	if err == nil {
		c.readCounter.Add(int64(buffer.Len()))
	}
	return
}

func (c *PacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	dataLen := int64(buffer.Len())
	err := c.PacketConn.WritePacket(buffer, destination)
	if err != nil {
		return err
	}
	c.writeCounter.Add(dataLen)
	return nil
}
