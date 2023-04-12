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
		origin:      socksaddrWithoutPort(origin),
		destination: socksaddrWithoutPort(destination),
	}
}

func (c *NATPacketConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	destination, err = c.PacketConn.ReadPacket(buffer)
	if socksaddrWithoutPort(destination) == c.origin {
		destination = M.Socksaddr{
			Addr: c.destination.Addr,
			Fqdn: c.destination.Fqdn,
			Port: destination.Port,
		}
	}
	return
}

func (c *NATPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	if socksaddrWithoutPort(destination) == c.destination {
		destination = M.Socksaddr{
			Addr: c.origin.Addr,
			Fqdn: c.origin.Fqdn,
			Port: destination.Port,
		}
	}
	return c.PacketConn.WritePacket(buffer, destination)
}

func (c *NATPacketConn) Upstream() any {
	return c.PacketConn
}

func socksaddrWithoutPort(destination M.Socksaddr) M.Socksaddr {
	destination.Port = 0
	return destination
}
