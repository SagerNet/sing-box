package canceler

import (
	"context"
	"time"

	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type PacketConn struct {
	N.PacketConn
	instance *Instance
}

func NewPacketConn(ctx context.Context, conn N.PacketConn, timeout time.Duration) (context.Context, N.PacketConn) {
	ctx, cancel := context.WithCancel(ctx)
	instance := New(ctx, cancel, timeout)
	return ctx, &PacketConn{conn, instance}
}

func (c *PacketConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	destination, err = c.PacketConn.ReadPacket(buffer)
	if err == nil {
		c.instance.Update()
	}
	return
}

func (c *PacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	err := c.PacketConn.WritePacket(buffer, destination)
	if err == nil {
		c.instance.Update()
	}
	return err
}

func (c *PacketConn) Upstream() any {
	return c.PacketConn
}
