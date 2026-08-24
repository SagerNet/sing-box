package iponly

import (
	"net"
	"os"

	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type PacketConn struct {
	N.NetPacketConn
	logger logger.Logger
}

func NewPacketConn(logger logger.Logger, conn net.PacketConn) *PacketConn {
	return &PacketConn{
		NetPacketConn: bufio.NewPacketConn(conn),
		logger:        logger,
	}
}

func (c *PacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	destination := M.SocksaddrFromNet(addr)
	if !destination.IsIP() {
		c.logger.Debug("dropped packet to non-IP destination ", destination)
		return len(p), nil
	}
	return c.NetPacketConn.WriteTo(p, addr)
}

func (c *PacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	if !destination.IsIP() {
		buffer.Release()
		c.logger.Debug("dropped packet to non-IP destination ", destination)
		return nil
	}
	return c.NetPacketConn.WritePacket(buffer, destination)
}

func (c *PacketConn) CreatePacketBatchWriter() (N.PacketBatchWriter, bool) {
	writer, created := bufio.CreatePacketBatchWriter(c.NetPacketConn)
	if !created {
		return nil, false
	}
	return &packetBatchWriter{c, writer}, true
}

func (c *PacketConn) CreateConnectedPacketBatchWriter() (N.ConnectedPacketBatchWriter, bool) {
	return bufio.CreateConnectedPacketBatchWriter(c.NetPacketConn)
}

func (c *PacketConn) ReaderReplaceable() bool {
	return true
}

func (c *PacketConn) Upstream() any {
	return c.NetPacketConn
}

type packetBatchWriter struct {
	*PacketConn
	writer N.PacketBatchWriter
}

func (w *packetBatchWriter) WritePacketBatch(buffers []*buf.Buffer, destinations []M.Socksaddr) error {
	if len(buffers) == 0 || len(buffers) != len(destinations) {
		buf.ReleaseMulti(buffers)
		return os.ErrInvalid
	}
	writeIndex := 0
	for index, destination := range destinations {
		if !destination.IsIP() {
			buffers[index].Release()
			w.logger.Debug("dropped packet to non-IP destination ", destination)
			continue
		}
		buffers[writeIndex] = buffers[index]
		destinations[writeIndex] = destination
		writeIndex++
	}
	if writeIndex == 0 {
		return nil
	}
	return w.writer.WritePacketBatch(buffers[:writeIndex], destinations[:writeIndex])
}

var (
	_ N.NetPacketConn                    = (*PacketConn)(nil)
	_ N.PacketBatchWriteCreator          = (*PacketConn)(nil)
	_ N.ConnectedPacketBatchWriteCreator = (*PacketConn)(nil)
)
