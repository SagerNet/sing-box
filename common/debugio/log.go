package debugio

import (
	"net"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type LogConn struct {
	N.ExtendedConn
	logger log.Logger
	prefix string
}

func NewLogConn(conn net.Conn, logger log.Logger, prefix string) N.ExtendedConn {
	return &LogConn{bufio.NewExtendedConn(conn), logger, prefix}
}

func (c *LogConn) Read(p []byte) (n int, err error) {
	n, err = c.ExtendedConn.Read(p)
	if n > 0 {
		c.logger.Debug(c.prefix, " read ", buf.EncodeHexString(p[:n]))
	}
	return
}

func (c *LogConn) Write(p []byte) (n int, err error) {
	c.logger.Debug(c.prefix, " write ", buf.EncodeHexString(p))
	return c.ExtendedConn.Write(p)
}

func (c *LogConn) ReadBuffer(buffer *buf.Buffer) error {
	err := c.ExtendedConn.ReadBuffer(buffer)
	if err == nil {
		c.logger.Debug(c.prefix, " read buffer ", buf.EncodeHexString(buffer.Bytes()))
	}
	return err
}

func (c *LogConn) WriteBuffer(buffer *buf.Buffer) error {
	c.logger.Debug(c.prefix, " write buffer ", buf.EncodeHexString(buffer.Bytes()))
	return c.ExtendedConn.WriteBuffer(buffer)
}

func (c *LogConn) Upstream() any {
	return c.ExtendedConn
}

type LogPacketConn struct {
	N.NetPacketConn
	logger log.Logger
	prefix string
}

func NewLogPacketConn(conn net.PacketConn, logger log.Logger, prefix string) N.NetPacketConn {
	return &LogPacketConn{bufio.NewPacketConn(conn), logger, prefix}
}

func (c *LogPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	n, addr, err = c.NetPacketConn.ReadFrom(p)
	if n > 0 {
		c.logger.Debug(c.prefix, " read from ", addr, " ", buf.EncodeHexString(p[:n]))
	}
	return
}

func (c *LogPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	c.logger.Debug(c.prefix, " write to ", addr, " ", buf.EncodeHexString(p))
	return c.NetPacketConn.WriteTo(p, addr)
}

func (c *LogPacketConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	destination, err = c.NetPacketConn.ReadPacket(buffer)
	if err == nil {
		c.logger.Debug(c.prefix, " read packet from ", destination, " ", buf.EncodeHexString(buffer.Bytes()))
	}
	return
}

func (c *LogPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	c.logger.Debug(c.prefix, " write packet to ", destination, " ", buf.EncodeHexString(buffer.Bytes()))
	return c.NetPacketConn.WritePacket(buffer, destination)
}
