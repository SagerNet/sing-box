package shadowtls

import (
	"encoding/binary"
	"io"
	"net"

	"github.com/jobberrt/sing-box/common/tls"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
)

var _ N.VectorisedWriter = (*Conn)(nil)

type Conn struct {
	net.Conn
	writer        N.VectorisedWriter
	readRemaining int
}

func NewConn(conn net.Conn) *Conn {
	return &Conn{
		Conn:   conn,
		writer: bufio.NewVectorisedWriter(conn),
	}
}

func (c *Conn) Read(p []byte) (n int, err error) {
	if c.readRemaining > 0 {
		if len(p) > c.readRemaining {
			p = p[:c.readRemaining]
		}
		n, err = c.Conn.Read(p)
		c.readRemaining -= n
		return
	}
	var tlsHeader [5]byte
	_, err = io.ReadFull(c.Conn, common.Dup(tlsHeader[:]))
	if err != nil {
		return
	}
	length := int(binary.BigEndian.Uint16(tlsHeader[3:5]))
	if tlsHeader[0] != 23 {
		return 0, E.New("unexpected TLS record type: ", tlsHeader[0])
	}
	readLen := len(p)
	if readLen > length {
		readLen = length
	}
	n, err = c.Conn.Read(p[:readLen])
	if err != nil {
		return
	}
	c.readRemaining = length - n
	return
}

func (c *Conn) Write(p []byte) (n int, err error) {
	var header [5]byte
	defer common.KeepAlive(header)
	header[0] = 23
	for len(p) > 16384 {
		binary.BigEndian.PutUint16(header[1:3], tls.VersionTLS12)
		binary.BigEndian.PutUint16(header[3:5], uint16(16384))
		_, err = bufio.WriteVectorised(c.writer, [][]byte{common.Dup(header[:]), p[:16384]})
		common.KeepAlive(header)
		if err != nil {
			return
		}
		n += 16384
		p = p[16384:]
	}
	binary.BigEndian.PutUint16(header[1:3], tls.VersionTLS12)
	binary.BigEndian.PutUint16(header[3:5], uint16(len(p)))
	_, err = bufio.WriteVectorised(c.writer, [][]byte{common.Dup(header[:]), p})
	if err == nil {
		n += len(p)
	}
	return
}

func (c *Conn) WriteVectorised(buffers []*buf.Buffer) error {
	var header [5]byte
	defer common.KeepAlive(header)
	header[0] = 23
	dataLen := buf.LenMulti(buffers)
	binary.BigEndian.PutUint16(header[1:3], tls.VersionTLS12)
	binary.BigEndian.PutUint16(header[3:5], uint16(dataLen))
	return c.writer.WriteVectorised(append([]*buf.Buffer{buf.As(header[:])}, buffers...))
}

func (c *Conn) Upstream() any {
	return c.Conn
}
