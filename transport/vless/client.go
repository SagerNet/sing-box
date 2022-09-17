package vless

import (
	"encoding/binary"
	"io"
	"net"

	"github.com/sagernet/sing-vmess"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/gofrs/uuid"
)

type Client struct {
	key []byte
}

func NewClient(userId string) (*Client, error) {
	user := uuid.FromStringOrNil(userId)
	if user == uuid.Nil {
		user = uuid.NewV5(user, userId)
	}
	return &Client{key: user.Bytes()}, nil
}

func (c *Client) DialEarlyConn(conn net.Conn, destination M.Socksaddr) *Conn {
	return &Conn{Conn: conn, key: c.key, command: vmess.CommandTCP, destination: destination}
}

func (c *Client) DialPacketConn(conn net.Conn, destination M.Socksaddr) *PacketConn {
	return &PacketConn{Conn: conn, key: c.key, destination: destination}
}

func (c *Client) DialXUDPPacketConn(conn net.Conn, destination M.Socksaddr) vmess.PacketConn {
	return vmess.NewXUDPConn(&Conn{Conn: conn, key: c.key, command: vmess.CommandMux, destination: destination}, destination)
}

type Conn struct {
	net.Conn
	key            []byte
	command        byte
	destination    M.Socksaddr
	requestWritten bool
	responseRead   bool
}

func (c *Conn) Read(b []byte) (n int, err error) {
	if !c.responseRead {
		err = ReadResponse(c.Conn)
		if err != nil {
			return
		}
		c.responseRead = true
	}
	return c.Conn.Read(b)
}

func (c *Conn) Write(b []byte) (n int, err error) {
	if !c.requestWritten {
		err = WriteRequest(c.Conn, Request{c.key, c.command, c.destination}, b)
		if err == nil {
			n = len(b)
		}
		c.requestWritten = true
		return
	}
	return c.Conn.Write(b)
}

func (c *Conn) Upstream() any {
	return c.Conn
}

type PacketConn struct {
	net.Conn
	key            []byte
	destination    M.Socksaddr
	requestWritten bool
	responseRead   bool
}

func (c *PacketConn) Read(b []byte) (n int, err error) {
	if !c.responseRead {
		err = ReadResponse(c.Conn)
		if err != nil {
			return
		}
		c.responseRead = true
	}
	var length uint16
	err = binary.Read(c.Conn, binary.BigEndian, &length)
	if err != nil {
		return
	}
	if cap(b) < int(length) {
		return 0, io.ErrShortBuffer
	}
	return io.ReadFull(c.Conn, b[:length])
}

func (c *PacketConn) Write(b []byte) (n int, err error) {
	if !c.requestWritten {
		err = WritePacketRequest(c.Conn, Request{c.key, vmess.CommandUDP, c.destination}, b)
		if err == nil {
			n = len(b)
		}
		c.requestWritten = true
		return
	}
	err = binary.Write(c.Conn, binary.BigEndian, uint16(len(b)))
	if err != nil {
		return
	}
	return c.Conn.Write(b)
}

func (c *PacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	defer buffer.Release()
	dataLen := buffer.Len()
	binary.BigEndian.PutUint16(buffer.ExtendHeader(2), uint16(dataLen))
	if !c.requestWritten {
		err := WritePacketRequest(c.Conn, Request{c.key, vmess.CommandUDP, c.destination}, buffer.Bytes())
		c.requestWritten = true
		return err
	}
	return common.Error(c.Conn.Write(buffer.Bytes()))
}

func (c *PacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	n, err = c.Read(p)
	return
}

func (c *PacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	return c.Write(p)
}

func (c *PacketConn) FrontHeadroom() int {
	return 2
}

func (c *PacketConn) Upstream() any {
	return c.Conn
}
