package http

import (
	std_bufio "bufio"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

const (
	capsuleTypeDatagram = 0
	capsuleHeadroom     = 1 + 4 + 1
	maxCapsuleLength    = 1 << 20
)

func readVarint(reader *std_bufio.Reader) (uint64, int, error) {
	first, err := reader.ReadByte()
	if err != nil {
		return 0, 0, err
	}
	length := 1 << (first >> 6)
	value := uint64(first & 0x3f)
	for i := 1; i < length; i++ {
		var next byte
		next, err = reader.ReadByte()
		if err != nil {
			return 0, 0, err
		}
		value = value<<8 | uint64(next)
	}
	return value, length, nil
}

func decodeVarint(b []byte) (uint64, int, bool) {
	if len(b) == 0 {
		return 0, 0, false
	}
	length := 1 << (b[0] >> 6)
	if len(b) < length {
		return 0, 0, false
	}
	value := uint64(b[0] & 0x3f)
	for i := 1; i < length; i++ {
		value = value<<8 | uint64(b[i])
	}
	return value, length, true
}

func varintLen(value uint64) int {
	switch {
	case value < 1<<6:
		return 1
	case value < 1<<14:
		return 2
	case value < 1<<30:
		return 4
	default:
		return 8
	}
}

func putVarint(b []byte, value uint64) int {
	switch varintLen(value) {
	case 1:
		b[0] = byte(value)
		return 1
	case 2:
		binary.BigEndian.PutUint16(b, uint16(value)|0x4000)
		return 2
	case 4:
		binary.BigEndian.PutUint32(b, uint32(value)|0x80000000)
		return 4
	default:
		binary.BigEndian.PutUint64(b, value|0xC000000000000000)
		return 8
	}
}

func readDatagramCapsule(reader *std_bufio.Reader, buffer *buf.Buffer) error {
	for {
		capsuleType, _, err := readVarint(reader)
		if err != nil {
			return err
		}
		length, _, err := readVarint(reader)
		if err != nil {
			return err
		}
		if length > maxCapsuleLength {
			return E.New("capsule too large: ", length)
		}
		if capsuleType != capsuleTypeDatagram {
			_, err = reader.Discard(int(length))
			if err != nil {
				return err
			}
			continue
		}
		contextID, contextLength, err := readVarint(reader)
		if err != nil {
			return err
		}
		if uint64(contextLength) > length {
			return E.New("malformed datagram capsule")
		}
		payloadLength := int(length) - contextLength
		if contextID != 0 || payloadLength > buffer.FreeLen() {
			_, err = reader.Discard(payloadLength)
			if err != nil {
				return err
			}
			continue
		}
		_, err = buffer.ReadFullFrom(reader, payloadLength)
		return err
	}
}

func prependContextID(buffer *buf.Buffer) *buf.Buffer {
	if buffer.Start() >= 1 {
		buffer.ExtendHeader(1)[0] = 0
		return buffer
	}
	datagram := buf.NewSize(1 + buffer.Len())
	datagram.WriteByte(0)
	datagram.Write(buffer.Bytes())
	buffer.Release()
	return datagram
}

func writeDatagramCapsule(writer io.Writer, datagram *buf.Buffer) error {
	length := uint64(datagram.Len())
	headerLength := 1 + varintLen(length)
	var capsule *buf.Buffer
	if datagram.Start() >= headerLength {
		header := datagram.ExtendHeader(headerLength)
		header[0] = capsuleTypeDatagram
		putVarint(header[1:], length)
		capsule = datagram
	} else {
		capsule = buf.NewSize(headerLength + datagram.Len())
		header := capsule.Extend(headerLength)
		header[0] = capsuleTypeDatagram
		putVarint(header[1:], length)
		capsule.Write(datagram.Bytes())
		datagram.Release()
	}
	defer capsule.Release()
	_, err := writer.Write(capsule.Bytes())
	return err
}

type capsuleConn struct {
	reader      *std_bufio.Reader
	writer      io.Writer
	upstream    net.Conn
	destination M.Socksaddr
	writeAccess sync.Mutex
}

func newCapsuleConn(reader *std_bufio.Reader, upstream net.Conn, destination M.Socksaddr) *capsuleConn {
	return &capsuleConn{
		reader:      reader,
		writer:      upstream,
		upstream:    upstream,
		destination: destination,
	}
}

func (c *capsuleConn) ReadPacket(buffer *buf.Buffer) (M.Socksaddr, error) {
	err := readDatagramCapsule(c.reader, buffer)
	if err != nil {
		return M.Socksaddr{}, err
	}
	return c.destination, nil
}

func (c *capsuleConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	c.writeAccess.Lock()
	defer c.writeAccess.Unlock()
	return writeDatagramCapsule(c.writer, prependContextID(buffer))
}

func (c *capsuleConn) Close() error {
	return c.upstream.Close()
}

func (c *capsuleConn) LocalAddr() net.Addr {
	return c.upstream.LocalAddr()
}

func (c *capsuleConn) SetDeadline(t time.Time) error {
	return c.upstream.SetDeadline(t)
}

func (c *capsuleConn) SetReadDeadline(t time.Time) error {
	return c.upstream.SetReadDeadline(t)
}

func (c *capsuleConn) SetWriteDeadline(t time.Time) error {
	return c.upstream.SetWriteDeadline(t)
}

func (c *capsuleConn) NeedAdditionalReadDeadline() bool {
	return true
}

func (c *capsuleConn) FrontHeadroom() int {
	return capsuleHeadroom
}

func (c *capsuleConn) Upstream() any {
	return c.upstream
}

var _ N.PacketConn = (*capsuleConn)(nil)

type DatagramStream interface {
	io.ReadWriteCloser
	SendDatagram(payload []byte) error
	ReceiveDatagram(ctx context.Context) ([]byte, error)
}

var (
	HTTP3StreamFunc        func(ctx context.Context, writer http.ResponseWriter) (DatagramStream, bool)
	ErrDatagramUnsupported = E.New("datagram unsupported")
)

type http3PacketConn struct {
	stream      DatagramStream
	reader      *std_bufio.Reader
	destination M.Socksaddr
	localAddr   net.Addr
	packets     chan *buf.Buffer
	ctx         context.Context
	cancel      context.CancelFunc
	closeOnce   sync.Once
	waitGroup   sync.WaitGroup
	writeAccess sync.Mutex
	err         error
}

func newHTTP3PacketConn(stream DatagramStream, destination M.Socksaddr, localAddr net.Addr) *http3PacketConn {
	ctx, cancel := context.WithCancel(context.Background())
	conn := &http3PacketConn{
		stream:      stream,
		reader:      std_bufio.NewReader(stream),
		destination: destination,
		localAddr:   localAddr,
		packets:     make(chan *buf.Buffer, 64),
		ctx:         ctx,
		cancel:      cancel,
	}
	conn.waitGroup.Add(2)
	go conn.loopDatagram()
	go conn.loopCapsule()
	return conn
}

func (c *http3PacketConn) loopDatagram() {
	defer c.waitGroup.Done()
	for {
		datagram, err := c.stream.ReceiveDatagram(c.ctx)
		if err != nil {
			c.closeWithError(err)
			return
		}
		contextID, contextLength, valid := decodeVarint(datagram)
		if !valid || contextID != 0 {
			continue
		}
		buffer := buf.NewSize(len(datagram) - contextLength)
		buffer.Write(datagram[contextLength:])
		select {
		case c.packets <- buffer:
		case <-c.ctx.Done():
			buffer.Release()
			return
		}
	}
}

func (c *http3PacketConn) loopCapsule() {
	defer c.waitGroup.Done()
	for {
		buffer := buf.NewPacket()
		err := readDatagramCapsule(c.reader, buffer)
		if err != nil {
			buffer.Release()
			c.closeWithError(err)
			return
		}
		select {
		case c.packets <- buffer:
		case <-c.ctx.Done():
			buffer.Release()
			return
		}
	}
}

func (c *http3PacketConn) ReadPacket(buffer *buf.Buffer) (M.Socksaddr, error) {
	for {
		select {
		case packet := <-c.packets:
			if packet.Len() > buffer.FreeLen() {
				packet.Release()
				continue
			}
			buffer.Write(packet.Bytes())
			packet.Release()
			return c.destination, nil
		case <-c.ctx.Done():
			return M.Socksaddr{}, c.err
		}
	}
}

func (c *http3PacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	datagram := prependContextID(buffer)
	err := c.stream.SendDatagram(datagram.Bytes())
	if err == nil {
		datagram.Release()
		return nil
	}
	if !errors.Is(err, ErrDatagramUnsupported) {
		datagram.Release()
		return err
	}
	c.writeAccess.Lock()
	defer c.writeAccess.Unlock()
	return writeDatagramCapsule(c.stream, datagram)
}

func (c *http3PacketConn) closeWithError(err error) {
	c.closeOnce.Do(func() {
		c.err = err
		c.cancel()
		c.stream.Close()
		go func() {
			c.waitGroup.Wait()
			for {
				select {
				case packet := <-c.packets:
					packet.Release()
				default:
					return
				}
			}
		}()
	})
}

func (c *http3PacketConn) Close() error {
	c.closeWithError(net.ErrClosed)
	return nil
}

func (c *http3PacketConn) wait(ctx context.Context) {
	select {
	case <-c.ctx.Done():
	case <-ctx.Done():
		c.Close()
	}
}

func (c *http3PacketConn) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *http3PacketConn) SetDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *http3PacketConn) SetReadDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *http3PacketConn) SetWriteDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *http3PacketConn) NeedAdditionalReadDeadline() bool {
	return true
}

func (c *http3PacketConn) FrontHeadroom() int {
	return capsuleHeadroom
}

var _ N.PacketConn = (*http3PacketConn)(nil)
