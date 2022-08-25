// Modified from: https://github.com/Qv2ray/gun-lite
// License: MIT

package v2raygrpc

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"ekyu.moe/leb128"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
)

var ErrInvalidLength = E.New("invalid length")

type GunConn struct {
	reader io.Reader
	writer io.Writer
	closer io.Closer
	// mu protect done
	mu   sync.Mutex
	done chan struct{}

	toRead []byte
	readAt int
}

func newGunConn(reader io.Reader, writer io.Writer, closer io.Closer) *GunConn {
	return &GunConn{
		reader: reader,
		writer: writer,
		closer: closer,
		done:   make(chan struct{}),
	}
}

func (c *GunConn) isClosed() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

func (c *GunConn) Read(b []byte) (n int, err error) {
	if c.toRead != nil {
		n = copy(b, c.toRead[c.readAt:])
		c.readAt += n
		if c.readAt >= len(c.toRead) {
			buf.Put(c.toRead)
			c.toRead = nil
		}
		return n, nil
	}
	buffer := buf.Get(5)
	n, err = io.ReadFull(c.reader, buffer)
	if err != nil {
		return 0, err
	}
	grpcPayloadLen := binary.BigEndian.Uint32(buffer[1:])
	buf.Put(buffer)

	buffer = buf.Get(int(grpcPayloadLen))
	n, err = io.ReadFull(c.reader, buffer)
	if err != nil {
		return 0, io.ErrUnexpectedEOF
	}
	protobufPayloadLen, protobufLengthLen := leb128.DecodeUleb128(buffer[1:])
	if protobufLengthLen == 0 {
		return 0, ErrInvalidLength
	}
	if grpcPayloadLen != uint32(protobufPayloadLen)+uint32(protobufLengthLen)+1 {
		return 0, ErrInvalidLength
	}
	n = copy(b, buffer[1+protobufLengthLen:])
	if n < int(protobufPayloadLen) {
		c.toRead = buffer
		c.readAt = 1 + int(protobufLengthLen) + n
		return n, nil
	}
	return n, nil
}

func (c *GunConn) Write(b []byte) (n int, err error) {
	if c.isClosed() {
		return 0, io.ErrClosedPipe
	}
	protobufHeader := leb128.AppendUleb128([]byte{0x0A}, uint64(len(b)))
	grpcHeader := buf.Get(5)
	grpcPayloadLen := uint32(len(protobufHeader) + len(b))
	binary.BigEndian.PutUint32(grpcHeader[1:5], grpcPayloadLen)
	_, err = bufio.Copy(c.writer, io.MultiReader(bytes.NewReader(grpcHeader), bytes.NewReader(protobufHeader), bytes.NewReader(b)))
	buf.Put(grpcHeader)
	if f, ok := c.writer.(http.Flusher); ok {
		f.Flush()
	}
	return len(b), err
}

func (c *GunConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	select {
	case <-c.done:
		return nil
	default:
		close(c.done)
		return c.closer.Close()
	}
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
