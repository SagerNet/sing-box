// Modified from: https://github.com/Qv2ray/gun-lite
// License: MIT

package v2raygrpclite

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
)

var ErrInvalidLength = E.New("invalid length")

var _ net.Conn = (*GunConn)(nil)

type GunConn struct {
	reader      io.Reader
	writer      io.Writer
	closer      io.Closer
	cached      []byte
	cachedIndex int
}

func newGunConn(reader io.Reader, writer io.Writer, closer io.Closer) *GunConn {
	return &GunConn{
		reader: reader,
		writer: writer,
		closer: closer,
	}
}

func (c *GunConn) Read(b []byte) (n int, err error) {
	if c.cached != nil {
		n = copy(b, c.cached[c.cachedIndex:])
		c.cachedIndex += n
		if c.cachedIndex == len(c.cached) {
			buf.Put(c.cached)
			c.cached = nil
		}
		return
	}
	buffer := buf.Get(5)
	_, err = io.ReadFull(c.reader, buffer)
	if err != nil {
		return 0, err
	}
	grpcPayloadLen := binary.BigEndian.Uint32(buffer[1:])
	buf.Put(buffer)

	buffer = buf.Get(int(grpcPayloadLen))
	_, err = io.ReadFull(c.reader, buffer)
	if err != nil {
		return 0, io.ErrUnexpectedEOF
	}
	protobufPayloadLen, protobufLengthLen := binary.Uvarint(buffer[1:])
	if protobufLengthLen == 0 {
		return 0, ErrInvalidLength
	}
	if grpcPayloadLen != uint32(protobufPayloadLen)+uint32(protobufLengthLen)+1 {
		return 0, ErrInvalidLength
	}
	n = copy(b, buffer[1+protobufLengthLen:])
	if n < int(protobufPayloadLen) {
		c.cached = buffer
		c.cachedIndex = 1 + int(protobufLengthLen) + n
		return n, nil
	}
	return n, nil
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
	return len(b), err
}

/*func (c *GunConn) ReadBuffer(buffer *buf.Buffer) error {
}

func (c *GunConn) WriteBuffer(buffer *buf.Buffer) error {
}*/

func (c *GunConn) Close() error {
	return c.closer.Close()
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
