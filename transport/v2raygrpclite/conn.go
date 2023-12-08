package v2raygrpclite

import (
	std_bufio "bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/baderror"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/rw"
)

// kanged from: https://github.com/Qv2ray/gun-lite

var _ net.Conn = (*GunConn)(nil)

type GunConn struct {
	reader        *std_bufio.Reader
	writer        io.Writer
	flusher       http.Flusher
	create        chan struct{}
	err           error
	readRemaining int
	writeAccess   sync.Mutex
}

func newGunConn(reader io.Reader, writer io.Writer, flusher http.Flusher) *GunConn {
	return &GunConn{
		reader:  std_bufio.NewReader(reader),
		writer:  writer,
		flusher: flusher,
	}
}

func newLateGunConn(writer io.Writer) *GunConn {
	return &GunConn{
		create: make(chan struct{}),
		writer: writer,
	}
}

func (c *GunConn) setup(reader io.Reader, err error) {
	if reader != nil {
		c.reader = std_bufio.NewReader(reader)
	}
	c.err = err
	close(c.create)
}

func (c *GunConn) Read(b []byte) (n int, err error) {
	n, err = c.read(b)
	return n, baderror.WrapH2(err)
}

func (c *GunConn) read(b []byte) (n int, err error) {
	if c.reader == nil {
		<-c.create
		if c.err != nil {
			return 0, c.err
		}
	}

	if c.readRemaining > 0 {
		if len(b) > c.readRemaining {
			b = b[:c.readRemaining]
		}
		n, err = c.reader.Read(b)
		c.readRemaining -= n
		return
	}

	_, err = c.reader.Discard(6)
	if err != nil {
		return
	}

	dataLen, err := binary.ReadUvarint(c.reader)
	if err != nil {
		return
	}

	readLen := int(dataLen)
	c.readRemaining = readLen
	if len(b) > readLen {
		b = b[:readLen]
	}

	n, err = c.reader.Read(b)
	c.readRemaining -= n
	return
}

func (c *GunConn) Write(b []byte) (n int, err error) {
	protobufHeader := [1 + binary.MaxVarintLen64]byte{0x0A}
	varuintLen := binary.PutUvarint(protobufHeader[1:], uint64(len(b)))
	grpcHeader := buf.Get(5)
	grpcPayloadLen := uint32(1 + varuintLen + len(b))
	binary.BigEndian.PutUint32(grpcHeader[1:5], grpcPayloadLen)
	c.writeAccess.Lock()
	_, err = bufio.Copy(c.writer, io.MultiReader(bytes.NewReader(grpcHeader), bytes.NewReader(protobufHeader[:varuintLen+1]), bytes.NewReader(b)))
	c.writeAccess.Unlock()
	buf.Put(grpcHeader)
	if err == nil && c.flusher != nil {
		c.flusher.Flush()
	}
	return len(b), baderror.WrapH2(err)
}

func (c *GunConn) WriteBuffer(buffer *buf.Buffer) error {
	defer buffer.Release()
	dataLen := buffer.Len()
	varLen := rw.UVariantLen(uint64(dataLen))
	header := buffer.ExtendHeader(6 + varLen)
	_ = header[6]
	header[0] = 0x00
	binary.BigEndian.PutUint32(header[1:5], uint32(1+varLen+dataLen))
	header[5] = 0x0A
	binary.PutUvarint(header[6:], uint64(dataLen))
	err := rw.WriteBytes(c.writer, buffer.Bytes())
	if err == nil && c.flusher != nil {
		c.flusher.Flush()
	}
	return baderror.WrapH2(err)
}

func (c *GunConn) FrontHeadroom() int {
	return 6 + binary.MaxVarintLen64
}

func (c *GunConn) Close() error {
	return common.Close(c.reader, c.writer)
}

func (c *GunConn) LocalAddr() net.Addr {
	return M.Socksaddr{}
}

func (c *GunConn) RemoteAddr() net.Addr {
	return M.Socksaddr{}
}

func (c *GunConn) SetDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *GunConn) SetReadDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *GunConn) SetWriteDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *GunConn) NeedAdditionalReadDeadline() bool {
	return true
}
