package v2raygrpclite

import (
	std_bufio "bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
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
	c.reader = std_bufio.NewReader(reader)
	c.err = err
	close(c.create)
}

func (c *GunConn) Read(b []byte) (n int, err error) {
	n, err = c.read(b)
	return n, wrapError(err)
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
	_, err = bufio.Copy(c.writer, io.MultiReader(bytes.NewReader(grpcHeader), bytes.NewReader(protobufHeader[:varuintLen+1]), bytes.NewReader(b)))
	buf.Put(grpcHeader)
	if f, ok := c.writer.(http.Flusher); ok {
		f.Flush()
	}
	return len(b), wrapError(err)
}

func uLen(x uint64) int {
	i := 0
	for x >= 0x80 {
		x >>= 7
		i++
	}
	return i + 1
}

func (c *GunConn) WriteBuffer(buffer *buf.Buffer) error {
	defer buffer.Release()
	dataLen := buffer.Len()
	varLen := uLen(uint64(dataLen))
	header := buffer.ExtendHeader(6 + varLen)
	binary.BigEndian.PutUint32(header[1:5], uint32(1+varLen+dataLen))
	header[5] = 0x0A
	binary.PutUvarint(header[6:], uint64(dataLen))
	err := rw.WriteBytes(c.writer, buffer.Bytes())
	if c.flusher != nil {
		c.flusher.Flush()
	}
	return wrapError(err)
}

func (c *GunConn) FrontHeadroom() int {
	return 6 + binary.MaxVarintLen64
}

func (c *GunConn) Close() error {
	return common.Close(c.reader, c.writer)
}

func (c *GunConn) LocalAddr() net.Addr {
	return nil
}

func (c *GunConn) RemoteAddr() net.Addr {
	return nil
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

func wrapError(err error) error {
	if E.IsMulti(err, io.ErrUnexpectedEOF) {
		return io.EOF
	}
	return err
}
