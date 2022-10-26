package hysteria

import (
	"crypto/sha256"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

const xplusSaltLen = 16

func NewXPlusPacketConn(conn net.PacketConn, key []byte) net.PacketConn {
	vectorisedWriter, isVectorised := bufio.CreateVectorisedPacketWriter(conn)
	if isVectorised {
		return &VectorisedXPlusConn{
			XPlusPacketConn: XPlusPacketConn{
				PacketConn: conn,
				key:        key,
				rand:       rand.New(rand.NewSource(time.Now().UnixNano())),
			},
			writer: vectorisedWriter,
		}
	} else {
		return &XPlusPacketConn{
			PacketConn: conn,
			key:        key,
			rand:       rand.New(rand.NewSource(time.Now().UnixNano())),
		}
	}
}

type XPlusPacketConn struct {
	net.PacketConn
	key        []byte
	randAccess sync.Mutex
	rand       *rand.Rand
}

func (c *XPlusPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	n, addr, err = c.PacketConn.ReadFrom(p)
	if err != nil {
		return
	} else if n < xplusSaltLen {
		n = 0
		return
	}
	key := sha256.Sum256(append(c.key, p[:xplusSaltLen]...))
	for i := range p[xplusSaltLen:] {
		p[i] = p[xplusSaltLen+i] ^ key[i%sha256.Size]
	}
	n -= xplusSaltLen
	return
}

func (c *XPlusPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	// can't use unsafe buffer on WriteTo
	buffer := buf.NewSize(len(p) + xplusSaltLen)
	defer buffer.Release()
	salt := buffer.Extend(xplusSaltLen)
	c.randAccess.Lock()
	_, _ = c.rand.Read(salt)
	c.randAccess.Unlock()
	key := sha256.Sum256(append(c.key, salt...))
	for i := range p {
		common.Must(buffer.WriteByte(p[i] ^ key[i%sha256.Size]))
	}
	return c.PacketConn.WriteTo(buffer.Bytes(), addr)
}

func (c *XPlusPacketConn) Upstream() any {
	return c.PacketConn
}

type VectorisedXPlusConn struct {
	XPlusPacketConn
	writer N.VectorisedPacketWriter
}

func (c *VectorisedXPlusConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	header := buf.NewSize(xplusSaltLen)
	defer header.Release()
	salt := header.Extend(xplusSaltLen)
	c.randAccess.Lock()
	_, _ = c.rand.Read(salt)
	c.randAccess.Unlock()
	key := sha256.Sum256(append(c.key, salt...))
	for i := range p {
		p[i] ^= key[i%sha256.Size]
	}
	return bufio.WriteVectorisedPacket(c.writer, [][]byte{header.Bytes(), p}, M.SocksaddrFromNet(addr))
}

func (c *VectorisedXPlusConn) WriteVectorisedPacket(buffers []*buf.Buffer, destination M.Socksaddr) error {
	header := buf.NewSize(xplusSaltLen)
	defer header.Release()
	salt := header.Extend(xplusSaltLen)
	c.randAccess.Lock()
	_, _ = c.rand.Read(salt)
	c.randAccess.Unlock()
	key := sha256.Sum256(append(c.key, salt...))
	var index int
	for _, buffer := range buffers {
		data := buffer.Bytes()
		for i := range data {
			data[i] ^= key[index%sha256.Size]
			index++
		}
	}
	buffers = append([]*buf.Buffer{header}, buffers...)
	return c.writer.WriteVectorisedPacket(buffers, destination)
}
