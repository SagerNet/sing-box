package fakeip

import (
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ N.PacketConn = (*NATPacketConn)(nil)

type NATPacketConn struct {
	N.PacketConn
	origin      M.Socksaddr
	destination M.Socksaddr
}

func NewNATPacketConn(conn N.PacketConn, origin M.Socksaddr, destination M.Socksaddr) *NATPacketConn {
	return &NATPacketConn{
		PacketConn:  conn,
		origin:      origin,
		destination: destination,
	}
}

func (c *NATPacketConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	destination, err = c.PacketConn.ReadPacket(buffer)
	if destination == c.origin {
		destination = c.destination
	}
	return
}

func (c *NATPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	if destination == c.destination {
		destination = c.origin
	}
	return c.PacketConn.WritePacket(buffer, destination)
}

func (c *NATPacketConn) Upstream() any {
	return c.PacketConn
}
