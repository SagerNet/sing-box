package trojan

import (
	"encoding/binary"

	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/rw"
)

var _ N.PacketReadWaiter = (*ClientPacketConn)(nil)

func (c *ClientPacketConn) InitializeReadWaiter(options N.ReadWaitOptions) (needCopy bool) {
	c.readWaitOptions = options
	return false
}

func (c *ClientPacketConn) WaitReadPacket() (buffer *buf.Buffer, destination M.Socksaddr, err error) {
	destination, err = M.SocksaddrSerializer.ReadAddrPort(c.Conn)
	if err != nil {
		return nil, M.Socksaddr{}, E.Cause(err, "read destination")
	}

	var length uint16
	err = binary.Read(c.Conn, binary.BigEndian, &length)
	if err != nil {
		return nil, M.Socksaddr{}, E.Cause(err, "read chunk length")
	}

	err = rw.SkipN(c.Conn, 2)
	if err != nil {
		return nil, M.Socksaddr{}, E.Cause(err, "skip crlf")
	}

	buffer = c.readWaitOptions.NewPacketBuffer()
	_, err = buffer.ReadFullFrom(c.Conn, int(length))
	if err != nil {
		buffer.Release()
		return
	}
	c.readWaitOptions.PostReturn(buffer)
	return
}
