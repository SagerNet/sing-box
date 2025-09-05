package trojan

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"net"
	"os"
	"sync"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/rw"
)

const (
	KeyLength  = 56
	CommandTCP = 1
	CommandUDP = 3
	CommandMux = 0x7f
)

var CRLF = []byte{'\r', '\n'}

var _ N.EarlyConn = (*ClientConn)(nil)

type ClientConn struct {
	N.ExtendedConn
	key           [KeyLength]byte
	destination   M.Socksaddr
	headerWritten bool
}

func NewClientConn(conn net.Conn, key [KeyLength]byte, destination M.Socksaddr) *ClientConn {
	return &ClientConn{
		ExtendedConn: bufio.NewExtendedConn(conn),
		key:          key,
		destination:  destination,
	}
}

func (c *ClientConn) NeedHandshake() bool {
	return !c.headerWritten
}

func (c *ClientConn) Write(p []byte) (n int, err error) {
	if c.headerWritten {
		return c.ExtendedConn.Write(p)
	}
	err = ClientHandshake(c.ExtendedConn, c.key, c.destination, p)
	if err != nil {
		return
	}
	n = len(p)
	c.headerWritten = true
	return
}

func (c *ClientConn) WriteBuffer(buffer *buf.Buffer) error {
	if c.headerWritten {
		return c.ExtendedConn.WriteBuffer(buffer)
	}
	err := ClientHandshakeBuffer(c.ExtendedConn, c.key, c.destination, buffer)
	if err != nil {
		return err
	}
	c.headerWritten = true
	return nil
}

func (c *ClientConn) FrontHeadroom() int {
	if !c.headerWritten {
		return KeyLength + 5 + M.MaxSocksaddrLength
	}
	return 0
}

func (c *ClientConn) Upstream() any {
	return c.ExtendedConn
}

type ClientPacketConn struct {
	net.Conn
	access          sync.Mutex
	key             [KeyLength]byte
	headerWritten   bool
	readWaitOptions N.ReadWaitOptions
}

func NewClientPacketConn(conn net.Conn, key [KeyLength]byte) *ClientPacketConn {
	return &ClientPacketConn{
		Conn: conn,
		key:  key,
	}
}

func (c *ClientPacketConn) NeedHandshake() bool {
	return !c.headerWritten
}

func (c *ClientPacketConn) ReadPacket(buffer *buf.Buffer) (M.Socksaddr, error) {
	return ReadPacket(c.Conn, buffer)
}

func (c *ClientPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	if !c.headerWritten {
		c.access.Lock()
		if c.headerWritten {
			c.access.Unlock()
		} else {
			err := ClientHandshakePacket(c.Conn, c.key, destination, buffer)
			c.headerWritten = true
			c.access.Unlock()
			return err
		}
	}
	return WritePacket(c.Conn, buffer, destination)
}

func (c *ClientPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	buffer := buf.With(p)
	destination, err := c.ReadPacket(buffer)
	if err != nil {
		return
	}
	n = buffer.Len()
	if destination.IsFqdn() {
		addr = destination
	} else {
		addr = destination.UDPAddr()
	}
	return
}

func (c *ClientPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	return bufio.WritePacket(c, p, addr)
}

func (c *ClientPacketConn) Read(p []byte) (n int, err error) {
	n, _, err = c.ReadFrom(p)
	return
}

func (c *ClientPacketConn) Write(p []byte) (n int, err error) {
	return 0, os.ErrInvalid
}

func (c *ClientPacketConn) FrontHeadroom() int {
	if !c.headerWritten {
		return KeyLength + 2*M.MaxSocksaddrLength + 9
	}
	return M.MaxSocksaddrLength + 4
}

func (c *ClientPacketConn) Upstream() any {
	return c.Conn
}

func Key(password string) [KeyLength]byte {
	var key [KeyLength]byte
	hash := sha256.New224()
	common.Must1(hash.Write([]byte(password)))
	hex.Encode(key[:], hash.Sum(nil))
	return key
}

func ClientHandshakeRaw(conn net.Conn, key [KeyLength]byte, command byte, destination M.Socksaddr, payload []byte) error {
	_, err := conn.Write(key[:])
	if err != nil {
		return err
	}
	_, err = conn.Write(CRLF)
	if err != nil {
		return err
	}
	_, err = conn.Write([]byte{command})
	if err != nil {
		return err
	}
	err = M.SocksaddrSerializer.WriteAddrPort(conn, destination)
	if err != nil {
		return err
	}
	_, err = conn.Write(CRLF)
	if err != nil {
		return err
	}
	if len(payload) > 0 {
		_, err = conn.Write(payload)
		if err != nil {
			return err
		}
	}
	return nil
}

func ClientHandshake(conn net.Conn, key [KeyLength]byte, destination M.Socksaddr, payload []byte) error {
	headerLen := KeyLength + M.SocksaddrSerializer.AddrPortLen(destination) + 5
	header := buf.NewSize(headerLen + len(payload))
	defer header.Release()
	common.Must1(header.Write(key[:]))
	common.Must1(header.Write(CRLF))
	common.Must(header.WriteByte(CommandTCP))
	err := M.SocksaddrSerializer.WriteAddrPort(header, destination)
	if err != nil {
		return err
	}
	common.Must1(header.Write(CRLF))
	common.Must1(header.Write(payload))
	_, err = conn.Write(header.Bytes())
	if err != nil {
		return E.Cause(err, "write request")
	}
	return nil
}

func ClientHandshakeBuffer(conn net.Conn, key [KeyLength]byte, destination M.Socksaddr, payload *buf.Buffer) error {
	header := buf.With(payload.ExtendHeader(KeyLength + M.SocksaddrSerializer.AddrPortLen(destination) + 5))
	common.Must1(header.Write(key[:]))
	common.Must1(header.Write(CRLF))
	common.Must(header.WriteByte(CommandTCP))
	err := M.SocksaddrSerializer.WriteAddrPort(header, destination)
	if err != nil {
		return err
	}
	common.Must1(header.Write(CRLF))

	_, err = conn.Write(payload.Bytes())
	if err != nil {
		return E.Cause(err, "write request")
	}
	return nil
}

func ClientHandshakePacket(conn net.Conn, key [KeyLength]byte, destination M.Socksaddr, payload *buf.Buffer) error {
	headerLen := KeyLength + 2*M.SocksaddrSerializer.AddrPortLen(destination) + 9
	payloadLen := payload.Len()
	var header *buf.Buffer
	var writeHeader bool
	if payload.Start() >= headerLen {
		header = buf.With(payload.ExtendHeader(headerLen))
	} else {
		header = buf.NewSize(headerLen)
		defer header.Release()
		writeHeader = true
	}
	common.Must1(header.Write(key[:]))
	common.Must1(header.Write(CRLF))
	common.Must(header.WriteByte(CommandUDP))
	err := M.SocksaddrSerializer.WriteAddrPort(header, destination)
	if err != nil {
		return err
	}
	common.Must1(header.Write(CRLF))
	common.Must(M.SocksaddrSerializer.WriteAddrPort(header, destination))
	common.Must(binary.Write(header, binary.BigEndian, uint16(payloadLen)))
	common.Must1(header.Write(CRLF))

	if writeHeader {
		_, err := conn.Write(header.Bytes())
		if err != nil {
			return E.Cause(err, "write request")
		}
	}

	_, err = conn.Write(payload.Bytes())
	if err != nil {
		return E.Cause(err, "write payload")
	}
	return nil
}

func ReadPacket(conn net.Conn, buffer *buf.Buffer) (M.Socksaddr, error) {
	destination, err := M.SocksaddrSerializer.ReadAddrPort(conn)
	if err != nil {
		return M.Socksaddr{}, E.Cause(err, "read destination")
	}

	var length uint16
	err = binary.Read(conn, binary.BigEndian, &length)
	if err != nil {
		return M.Socksaddr{}, E.Cause(err, "read chunk length")
	}

	err = rw.SkipN(conn, 2)
	if err != nil {
		return M.Socksaddr{}, E.Cause(err, "skip crlf")
	}

	_, err = buffer.ReadFullFrom(conn, int(length))
	return destination, err
}

func WritePacket(conn net.Conn, buffer *buf.Buffer, destination M.Socksaddr) error {
	defer buffer.Release()
	bufferLen := buffer.Len()
	header := buf.With(buffer.ExtendHeader(M.SocksaddrSerializer.AddrPortLen(destination) + 4))
	err := M.SocksaddrSerializer.WriteAddrPort(header, destination)
	if err != nil {
		return err
	}
	common.Must(binary.Write(header, binary.BigEndian, uint16(bufferLen)))
	common.Must1(header.Write(CRLF))
	_, err = conn.Write(buffer.Bytes())
	if err != nil {
		return E.Cause(err, "write packet")
	}
	return nil
}
