package hysteria2

import (
	"net"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"golang.org/x/crypto/blake2b"
)

const salamanderSaltLen = 8

const ObfsTypeSalamander = "salamander"

type Salamander struct {
	net.PacketConn
	password []byte
}

func NewSalamanderConn(conn net.PacketConn, password []byte) net.PacketConn {
	writer, isVectorised := bufio.CreateVectorisedPacketWriter(conn)
	if isVectorised {
		return &VectorisedSalamander{
			Salamander: Salamander{
				PacketConn: conn,
				password:   password,
			},
			writer: writer,
		}
	} else {
		return &Salamander{
			PacketConn: conn,
			password:   password,
		}
	}
}

func (s *Salamander) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	n, addr, err = s.PacketConn.ReadFrom(p)
	if err != nil {
		return
	}
	if n <= salamanderSaltLen {
		return 0, nil, E.New("salamander: packet too short")
	}
	key := blake2b.Sum256(append(s.password, p[:salamanderSaltLen]...))
	for index, c := range p[salamanderSaltLen:n] {
		p[index] = c ^ key[index%blake2b.Size256]
	}
	return n - salamanderSaltLen, addr, nil
}

func (s *Salamander) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	buffer := buf.NewSize(len(p) + salamanderSaltLen)
	defer buffer.Release()
	buffer.WriteRandom(salamanderSaltLen)
	key := blake2b.Sum256(append(s.password, buffer.Bytes()...))
	for index, c := range p {
		common.Must(buffer.WriteByte(c ^ key[index%blake2b.Size256]))
	}
	_, err = s.PacketConn.WriteTo(buffer.Bytes(), addr)
	if err != nil {
		return
	}
	return len(p), nil
}

type VectorisedSalamander struct {
	Salamander
	writer N.VectorisedPacketWriter
}

func (s *VectorisedSalamander) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	buffer := buf.NewSize(salamanderSaltLen)
	buffer.WriteRandom(salamanderSaltLen)
	key := blake2b.Sum256(append(s.password, buffer.Bytes()...))
	for i := range p {
		p[i] ^= key[i%blake2b.Size256]
	}
	err = s.writer.WriteVectorisedPacket([]*buf.Buffer{buffer, buf.As(p)}, M.SocksaddrFromNet(addr))
	if err != nil {
		return
	}
	return len(p), nil
}

func (s *VectorisedSalamander) WriteVectorisedPacket(buffers []*buf.Buffer, destination M.Socksaddr) error {
	header := buf.NewSize(salamanderSaltLen)
	defer header.Release()
	header.WriteRandom(salamanderSaltLen)
	key := blake2b.Sum256(append(s.password, header.Bytes()...))
	var bufferIndex int
	for _, buffer := range buffers {
		content := buffer.Bytes()
		for index, c := range content {
			content[bufferIndex+index] = c ^ key[bufferIndex+index%blake2b.Size256]
		}
		bufferIndex += len(content)
	}
	return s.writer.WriteVectorisedPacket(append([]*buf.Buffer{header}, buffers...), destination)
}
