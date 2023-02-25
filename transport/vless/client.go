package vless

import (
	"encoding/binary"
	"io"
	"net"

	"github.com/sagernet/sing-vmess"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/gofrs/uuid"
)

type Client struct {
	key  [16]byte
	flow string
}

func NewClient(userId string, flow string) (*Client, error) {
	user := uuid.FromStringOrNil(userId)
	if user == uuid.Nil {
		user = uuid.NewV5(user, userId)
	}
	switch flow {
	case "", "xtls-rprx-vision":
	default:
		return nil, E.New("unsupported flow: " + flow)
	}
	return &Client{user, flow}, nil
}

func (c *Client) prepareConn(conn net.Conn) (net.Conn, error) {
	if c.flow == FlowVision {
		vConn, err := NewVisionConn(conn, c.key)
		if err != nil {
			return nil, E.Cause(err, "initialize vision")
		}
		conn = vConn
	}
	return conn, nil
}

func (c *Client) DialConn(conn net.Conn, destination M.Socksaddr) (*Conn, error) {
	vConn, err := c.prepareConn(conn)
	if err != nil {
		return nil, err
	}
	serverConn := &Conn{Conn: conn, protocolConn: vConn, key: c.key, command: vmess.CommandTCP, destination: destination, flow: c.flow}
	return serverConn, common.Error(serverConn.Write(nil))
}

func (c *Client) DialEarlyConn(conn net.Conn, destination M.Socksaddr) (*Conn, error) {
	vConn, err := c.prepareConn(conn)
	if err != nil {
		return nil, err
	}
	return &Conn{Conn: conn, protocolConn: vConn, key: c.key, command: vmess.CommandTCP, destination: destination, flow: c.flow}, nil
}

func (c *Client) DialPacketConn(conn net.Conn, destination M.Socksaddr) (*PacketConn, error) {
	serverConn := &PacketConn{Conn: conn, key: c.key, destination: destination, flow: c.flow}
	return serverConn, common.Error(serverConn.Write(nil))
}

func (c *Client) DialEarlyPacketConn(conn net.Conn, destination M.Socksaddr) (*PacketConn, error) {
	return &PacketConn{Conn: conn, key: c.key, destination: destination, flow: c.flow}, nil
}

func (c *Client) DialXUDPPacketConn(conn net.Conn, destination M.Socksaddr) (vmess.PacketConn, error) {
	serverConn := &Conn{Conn: conn, protocolConn: conn, key: c.key, command: vmess.CommandMux, destination: destination, flow: c.flow}
	err := common.Error(serverConn.Write(nil))
	if err != nil {
		return nil, err
	}
	return vmess.NewXUDPConn(serverConn, destination), nil
}

func (c *Client) DialEarlyXUDPPacketConn(conn net.Conn, destination M.Socksaddr) (vmess.PacketConn, error) {
	return vmess.NewXUDPConn(&Conn{Conn: conn, protocolConn: conn, key: c.key, command: vmess.CommandMux, destination: destination, flow: c.flow}, destination), nil
}

type Conn struct {
	net.Conn
	protocolConn   net.Conn
	key            [16]byte
	command        byte
	destination    M.Socksaddr
	flow           string
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
	return c.protocolConn.Read(b)
}

func (c *Conn) Write(b []byte) (n int, err error) {
	if !c.requestWritten {
		request := Request{c.key, c.command, c.destination, c.flow}
		if c.protocolConn != nil {
			err = WriteRequest(c.Conn, request, nil)
		} else {
			err = WriteRequest(c.Conn, request, b)
		}
		if err == nil {
			n = len(b)
		}
		c.requestWritten = true
		if c.protocolConn == nil {
			return
		}
	}
	return c.protocolConn.Write(b)
}

func (c *Conn) Upstream() any {
	return c.Conn
}

type PacketConn struct {
	net.Conn
	key            [16]byte
	destination    M.Socksaddr
	flow           string
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
		err = WritePacketRequest(c.Conn, Request{c.key, vmess.CommandUDP, c.destination, c.flow}, nil)
		if err == nil {
			n = len(b)
		}
		c.requestWritten = true
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
		err := WritePacketRequest(c.Conn, Request{c.key, vmess.CommandUDP, c.destination, c.flow}, buffer.Bytes())
		c.requestWritten = true
		return err
	}
	return common.Error(c.Conn.Write(buffer.Bytes()))
}

func (c *PacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	n, err = c.Read(p)
	if err != nil {
		return
	}
	addr = c.destination.UDPAddr()
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
