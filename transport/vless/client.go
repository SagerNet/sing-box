package vless

import (
	"encoding/binary"
	"io"
	"net"
	"sync"

	"github.com/sagernet/sing-vmess"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/gofrs/uuid/v5"
)

type Client struct {
	key    [16]byte
	flow   string
	logger logger.Logger
}

func NewClient(userId string, flow string, logger logger.Logger) (*Client, error) {
	user := uuid.FromStringOrNil(userId)
	if user == uuid.Nil {
		user = uuid.NewV5(user, userId)
	}
	switch flow {
	case "", "xtls-rprx-vision":
	default:
		return nil, E.New("unsupported flow: " + flow)
	}
	return &Client{user, flow, logger}, nil
}

func (c *Client) prepareConn(conn net.Conn, tlsConn net.Conn) (net.Conn, error) {
	if c.flow == FlowVision {
		protocolConn, err := NewVisionConn(conn, tlsConn, c.key, c.logger)
		if err != nil {
			return nil, E.Cause(err, "initialize vision")
		}
		conn = protocolConn
	}
	return conn, nil
}

func (c *Client) DialConn(conn net.Conn, destination M.Socksaddr) (net.Conn, error) {
	remoteConn := NewConn(conn, c.key, vmess.CommandTCP, destination, c.flow)
	protocolConn, err := c.prepareConn(remoteConn, conn)
	if err != nil {
		return nil, err
	}
	return protocolConn, common.Error(remoteConn.Write(nil))
}

func (c *Client) DialEarlyConn(conn net.Conn, destination M.Socksaddr) (net.Conn, error) {
	return c.prepareConn(NewConn(conn, c.key, vmess.CommandTCP, destination, c.flow), conn)
}

func (c *Client) DialPacketConn(conn net.Conn, destination M.Socksaddr) (*PacketConn, error) {
	serverConn := &PacketConn{Conn: conn, key: c.key, destination: destination, flow: c.flow}
	return serverConn, common.Error(serverConn.Write(nil))
}

func (c *Client) DialEarlyPacketConn(conn net.Conn, destination M.Socksaddr) (*PacketConn, error) {
	return &PacketConn{Conn: conn, key: c.key, destination: destination, flow: c.flow}, nil
}

func (c *Client) DialXUDPPacketConn(conn net.Conn, destination M.Socksaddr) (vmess.PacketConn, error) {
	remoteConn := NewConn(conn, c.key, vmess.CommandTCP, destination, c.flow)
	protocolConn, err := c.prepareConn(remoteConn, conn)
	if err != nil {
		return nil, err
	}
	return vmess.NewXUDPConn(protocolConn, destination), common.Error(remoteConn.Write(nil))
}

func (c *Client) DialEarlyXUDPPacketConn(conn net.Conn, destination M.Socksaddr) (vmess.PacketConn, error) {
	remoteConn := NewConn(conn, c.key, vmess.CommandMux, destination, c.flow)
	protocolConn, err := c.prepareConn(remoteConn, conn)
	if err != nil {
		return nil, err
	}
	return vmess.NewXUDPConn(protocolConn, destination), common.Error(remoteConn.Write(nil))
}

var (
	_ N.EarlyConn        = (*Conn)(nil)
	_ N.VectorisedWriter = (*Conn)(nil)
)

type Conn struct {
	N.ExtendedConn
	writer         N.VectorisedWriter
	request        Request
	requestWritten bool
	responseRead   bool
}

func NewConn(conn net.Conn, uuid [16]byte, command byte, destination M.Socksaddr, flow string) *Conn {
	return &Conn{
		ExtendedConn: bufio.NewExtendedConn(conn),
		writer:       bufio.NewVectorisedWriter(conn),
		request: Request{
			UUID:        uuid,
			Command:     command,
			Destination: destination,
			Flow:        flow,
		},
	}
}

func (c *Conn) Read(b []byte) (n int, err error) {
	if !c.responseRead {
		err = ReadResponse(c.ExtendedConn)
		if err != nil {
			return
		}
		c.responseRead = true
	}
	return c.ExtendedConn.Read(b)
}

func (c *Conn) ReadBuffer(buffer *buf.Buffer) error {
	if !c.responseRead {
		err := ReadResponse(c.ExtendedConn)
		if err != nil {
			return err
		}
		c.responseRead = true
	}
	return c.ExtendedConn.ReadBuffer(buffer)
}

func (c *Conn) Write(b []byte) (n int, err error) {
	if !c.requestWritten {
		err = WriteRequest(c.ExtendedConn, c.request, b)
		if err == nil {
			n = len(b)
		}
		c.requestWritten = true
		return
	}
	return c.ExtendedConn.Write(b)
}

func (c *Conn) WriteBuffer(buffer *buf.Buffer) error {
	if !c.requestWritten {
		err := EncodeRequest(c.request, buf.With(buffer.ExtendHeader(RequestLen(c.request))))
		if err != nil {
			return err
		}
		c.requestWritten = true
	}
	return c.ExtendedConn.WriteBuffer(buffer)
}

func (c *Conn) WriteVectorised(buffers []*buf.Buffer) error {
	if !c.requestWritten {
		buffer := buf.NewSize(RequestLen(c.request))
		err := EncodeRequest(c.request, buffer)
		if err != nil {
			buffer.Release()
			return err
		}
		c.requestWritten = true
		return c.writer.WriteVectorised(append([]*buf.Buffer{buffer}, buffers...))
	}
	return c.writer.WriteVectorised(buffers)
}

func (c *Conn) ReaderReplaceable() bool {
	return c.responseRead
}

func (c *Conn) WriterReplaceable() bool {
	return c.requestWritten
}

func (c *Conn) NeedHandshake() bool {
	return !c.requestWritten
}

func (c *Conn) FrontHeadroom() int {
	if c.requestWritten {
		return 0
	}
	return RequestLen(c.request)
}

func (c *Conn) Upstream() any {
	return c.ExtendedConn
}

type PacketConn struct {
	net.Conn
	access         sync.Mutex
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
		c.access.Lock()
		if c.requestWritten {
			c.access.Unlock()
		} else {
			err = WritePacketRequest(c.Conn, Request{c.key, vmess.CommandUDP, c.destination, c.flow}, nil)
			if err == nil {
				n = len(b)
			}
			c.requestWritten = true
			c.access.Unlock()
		}
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
		c.access.Lock()
		if c.requestWritten {
			c.access.Unlock()
		} else {
			err := WritePacketRequest(c.Conn, Request{c.key, vmess.CommandUDP, c.destination, c.flow}, buffer.Bytes())
			c.requestWritten = true
			c.access.Unlock()
			return err
		}
	}
	return common.Error(c.Conn.Write(buffer.Bytes()))
}

func (c *PacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	n, err = c.Read(p)
	if err != nil {
		return
	}
	if c.destination.IsFqdn() {
		addr = c.destination
	} else {
		addr = c.destination.UDPAddr()
	}
	return
}

func (c *PacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	return c.Write(p)
}

func (c *PacketConn) FrontHeadroom() int {
	return 2
}

func (c *PacketConn) NeedAdditionalReadDeadline() bool {
	return true
}

func (c *PacketConn) Upstream() any {
	return c.Conn
}
