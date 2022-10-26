package trackerconn

import (
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"go.uber.org/atomic"
)

func NewPacket(conn N.PacketConn, readCounter []*atomic.Int64, writeCounter []*atomic.Int64) *PacketConn {
	return &PacketConn{conn, readCounter, writeCounter}
}

func NewHookPacket(conn N.PacketConn, readCounter func(n int64), writeCounter func(n int64)) *HookPacketConn {
	return &HookPacketConn{conn, readCounter, writeCounter}
}

type PacketConn struct {
	N.PacketConn
	readCounter  []*atomic.Int64
	writeCounter []*atomic.Int64
}

func (c *PacketConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	destination, err = c.PacketConn.ReadPacket(buffer)
	if err == nil {
		for _, counter := range c.readCounter {
			counter.Add(int64(buffer.Len()))
		}
	}
	return
}

func (c *PacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	dataLen := int64(buffer.Len())
	err := c.PacketConn.WritePacket(buffer, destination)
	if err != nil {
		return err
	}
	for _, counter := range c.writeCounter {
		counter.Add(dataLen)
	}
	return nil
}

func (c *PacketConn) Upstream() any {
	return c.PacketConn
}

type HookPacketConn struct {
	N.PacketConn
	readCounter  func(n int64)
	writeCounter func(n int64)
}

func (c *HookPacketConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	destination, err = c.PacketConn.ReadPacket(buffer)
	if err == nil {
		c.readCounter(int64(buffer.Len()))
	}
	return
}

func (c *HookPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	dataLen := int64(buffer.Len())
	err := c.PacketConn.WritePacket(buffer, destination)
	if err != nil {
		return err
	}
	c.writeCounter(dataLen)
	return nil
}

func (c *HookPacketConn) Upstream() any {
	return c.PacketConn
}
