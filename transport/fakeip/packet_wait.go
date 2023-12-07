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
	readWaiter N.PacketReadWaiter
}

func (c *waitNATPacketConn) InitializeReadWaiter(options N.ReadWaitOptions) (needCopy bool) {
	return c.readWaiter.InitializeReadWaiter(options)
}

func (c *waitNATPacketConn) WaitReadPacket() (buffer *buf.Buffer, destination M.Socksaddr, err error) {
	buffer, destination, err = c.readWaiter.WaitReadPacket()
	if err == nil && socksaddrWithoutPort(destination) == c.origin {
		destination = M.Socksaddr{
			Addr: c.destination.Addr,
			Fqdn: c.destination.Fqdn,
			Port: destination.Port,
		}
	}
	return
}
