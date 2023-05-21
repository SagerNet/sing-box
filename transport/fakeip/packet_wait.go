package fakeip

import (
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func (c *NATPacketConn) CreatePacketReadWaiter() (N.PacketReadWaiter, bool) {
	waiter, created := bufio.CreatePacketReadWaiter(c.PacketConn)
	if !created {
		return nil, false
	}
	return &waitNATPacketConn{c, waiter}, true
}

type waitNATPacketConn struct {
	*NATPacketConn
	waiter N.PacketReadWaiter
}

func (c *waitNATPacketConn) InitializeReadWaiter(newBuffer func() *buf.Buffer) {
	c.waiter.InitializeReadWaiter(newBuffer)
}

func (c *waitNATPacketConn) WaitReadPacket() (destination M.Socksaddr, err error) {
	destination, err = c.waiter.WaitReadPacket()
	if socksaddrWithoutPort(destination) == c.origin {
		destination = M.Socksaddr{
			Addr: c.destination.Addr,
			Fqdn: c.destination.Fqdn,
			Port: destination.Port,
		}
	}
	return
}
