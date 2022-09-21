package mux

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"sync"

	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/x/list"
)

var _ N.Dialer = (*Client)(nil)

type Client struct {
	access         sync.Mutex
	connections    list.List[abstractSession]
	ctx            context.Context
	dialer         N.Dialer
	protocol       Protocol
	maxConnections int
	minStreams     int
	maxStreams     int
}

func NewClient(ctx context.Context, dialer N.Dialer, protocol Protocol, maxConnections int, minStreams int, maxStreams int) *Client {
	return &Client{
		ctx:            ctx,
		dialer:         dialer,
		protocol:       protocol,
		maxConnections: maxConnections,
		minStreams:     minStreams,
		maxStreams:     maxStreams,
	}
}

func NewClientWithOptions(ctx context.Context, dialer N.Dialer, options option.MultiplexOptions) (N.Dialer, error) {
	if !options.Enabled {
		return nil, nil
	}
	if options.MaxConnections == 0 && options.MaxStreams == 0 {
		options.MinStreams = 8
	}
	protocol, err := ParseProtocol(options.Protocol)
	if err != nil {
		return nil, err
	}
	return NewClient(ctx, dialer, protocol, options.MaxConnections, options.MinStreams, options.MaxStreams), nil
}

func (c *Client) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		stream, err := c.openStream()
		if err != nil {
			return nil, err
		}
		return &ClientConn{Conn: stream, destination: destination}, nil
	case N.NetworkUDP:
		stream, err := c.openStream()
		if err != nil {
			return nil, err
		}
		return bufio.NewUnbindPacketConn(&ClientPacketConn{ExtendedConn: bufio.NewExtendedConn(stream), destination: destination}), nil
	default:
		return nil, E.Extend(N.ErrUnknownNetwork, network)
	}
}

func (c *Client) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	stream, err := c.openStream()
	if err != nil {
		return nil, err
	}
	return &ClientPacketAddrConn{ExtendedConn: bufio.NewExtendedConn(stream), destination: destination}, nil
}

func (c *Client) openStream() (net.Conn, error) {
	var (
		session abstractSession
		stream  net.Conn
		err     error
	)
	for attempts := 0; attempts < 2; attempts++ {
		session, err = c.offer()
		if err != nil {
			continue
		}
		stream, err = session.Open()
		if err != nil {
			continue
		}
		break
	}
	if err != nil {
		return nil, err
	}
	return &wrapStream{stream}, nil
}

func (c *Client) offer() (abstractSession, error) {
	c.access.Lock()
	defer c.access.Unlock()

	sessions := make([]abstractSession, 0, c.maxConnections)
	for element := c.connections.Front(); element != nil; {
		if element.Value.IsClosed() {
			nextElement := element.Next()
			c.connections.Remove(element)
			element = nextElement
			continue
		}
		sessions = append(sessions, element.Value)
		element = element.Next()
	}
	sLen := len(sessions)
	if sLen == 0 {
		return c.offerNew()
	}
	session := common.MinBy(sessions, abstractSession.NumStreams)
	numStreams := session.NumStreams()
	if numStreams == 0 {
		return session, nil
	}
	if c.maxConnections > 0 {
		if sLen >= c.maxConnections || numStreams < c.minStreams {
			return session, nil
		}
	} else {
		if c.maxStreams > 0 && numStreams < c.maxStreams {
			return session, nil
		}
	}
	return c.offerNew()
}

func (c *Client) offerNew() (abstractSession, error) {
	conn, err := c.dialer.DialContext(c.ctx, N.NetworkTCP, Destination)
	if err != nil {
		return nil, err
	}
	if vectorisedWriter, isVectorised := bufio.CreateVectorisedWriter(conn); isVectorised {
		conn = &vectorisedProtocolConn{protocolConn{Conn: conn, protocol: c.protocol}, vectorisedWriter}
	} else {
		conn = &protocolConn{Conn: conn, protocol: c.protocol}
	}
	session, err := c.protocol.newClient(conn)
	if err != nil {
		return nil, err
	}
	c.connections.PushBack(session)
	return session, nil
}

func (c *Client) Close() error {
	c.access.Lock()
	defer c.access.Unlock()
	for _, session := range c.connections.Array() {
		session.Close()
	}
	return nil
}

type ClientConn struct {
	net.Conn
	destination  M.Socksaddr
	requestWrite bool
	responseRead bool
}

func (c *ClientConn) readResponse() error {
	response, err := ReadStreamResponse(c.Conn)
	if err != nil {
		return err
	}
	if response.Status == statusError {
		return E.New("remote error: ", response.Message)
	}
	return nil
}

func (c *ClientConn) Read(b []byte) (n int, err error) {
	if !c.responseRead {
		err = c.readResponse()
		if err != nil {
			return
		}
		c.responseRead = true
	}
	return c.Conn.Read(b)
}

func (c *ClientConn) Write(b []byte) (n int, err error) {
	if c.requestWrite {
		return c.Conn.Write(b)
	}
	request := StreamRequest{
		Network:     N.NetworkTCP,
		Destination: c.destination,
	}
	_buffer := buf.StackNewSize(requestLen(request) + len(b))
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	EncodeStreamRequest(request, buffer)
	buffer.Write(b)
	_, err = c.Conn.Write(buffer.Bytes())
	if err != nil {
		return
	}
	c.requestWrite = true
	return len(b), nil
}

func (c *ClientConn) ReadFrom(r io.Reader) (n int64, err error) {
	if !c.requestWrite {
		return bufio.ReadFrom0(c, r)
	}
	return bufio.Copy(c.Conn, r)
}

func (c *ClientConn) WriteTo(w io.Writer) (n int64, err error) {
	if !c.responseRead {
		return bufio.WriteTo0(c, w)
	}
	return bufio.Copy(w, c.Conn)
}

func (c *ClientConn) LocalAddr() net.Addr {
	return c.Conn.LocalAddr()
}

func (c *ClientConn) RemoteAddr() net.Addr {
	return c.destination.TCPAddr()
}

func (c *ClientConn) ReaderReplaceable() bool {
	return c.responseRead
}

func (c *ClientConn) WriterReplaceable() bool {
	return c.requestWrite
}

func (c *ClientConn) Upstream() any {
	return c.Conn
}

type ClientPacketConn struct {
	N.ExtendedConn
	destination  M.Socksaddr
	requestWrite bool
	responseRead bool
}

func (c *ClientPacketConn) readResponse() error {
	response, err := ReadStreamResponse(c.ExtendedConn)
	if err != nil {
		return err
	}
	if response.Status == statusError {
		return E.New("remote error: ", response.Message)
	}
	return nil
}

func (c *ClientPacketConn) Read(b []byte) (n int, err error) {
	if !c.responseRead {
		err = c.readResponse()
		if err != nil {
			return
		}
		c.responseRead = true
	}
	var length uint16
	err = binary.Read(c.ExtendedConn, binary.BigEndian, &length)
	if err != nil {
		return
	}
	if cap(b) < int(length) {
		return 0, io.ErrShortBuffer
	}
	return io.ReadFull(c.ExtendedConn, b[:length])
}

func (c *ClientPacketConn) writeRequest(payload []byte) (n int, err error) {
	request := StreamRequest{
		Network:     N.NetworkUDP,
		Destination: c.destination,
	}
	rLen := requestLen(request)
	if len(payload) > 0 {
		rLen += 2 + len(payload)
	}
	_buffer := buf.StackNewSize(rLen)
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	EncodeStreamRequest(request, buffer)
	if len(payload) > 0 {
		common.Must(
			binary.Write(buffer, binary.BigEndian, uint16(len(payload))),
			common.Error(buffer.Write(payload)),
		)
	}
	_, err = c.ExtendedConn.Write(buffer.Bytes())
	if err != nil {
		return
	}
	c.requestWrite = true
	return len(payload), nil
}

func (c *ClientPacketConn) Write(b []byte) (n int, err error) {
	if !c.requestWrite {
		return c.writeRequest(b)
	}
	err = binary.Write(c.ExtendedConn, binary.BigEndian, uint16(len(b)))
	if err != nil {
		return
	}
	return c.ExtendedConn.Write(b)
}

func (c *ClientPacketConn) WriteBuffer(buffer *buf.Buffer) error {
	if !c.requestWrite {
		defer buffer.Release()
		return common.Error(c.writeRequest(buffer.Bytes()))
	}
	bLen := buffer.Len()
	binary.BigEndian.PutUint16(buffer.ExtendHeader(2), uint16(bLen))
	return c.ExtendedConn.WriteBuffer(buffer)
}

func (c *ClientPacketConn) FrontHeadroom() int {
	return 2
}

func (c *ClientPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	return c.WriteBuffer(buffer)
}

func (c *ClientPacketConn) LocalAddr() net.Addr {
	return c.ExtendedConn.LocalAddr()
}

func (c *ClientPacketConn) RemoteAddr() net.Addr {
	return c.destination.UDPAddr()
}

func (c *ClientPacketConn) Upstream() any {
	return c.ExtendedConn
}

var _ N.NetPacketConn = (*ClientPacketAddrConn)(nil)

type ClientPacketAddrConn struct {
	N.ExtendedConn
	destination  M.Socksaddr
	requestWrite bool
	responseRead bool
}

func (c *ClientPacketAddrConn) readResponse() error {
	response, err := ReadStreamResponse(c.ExtendedConn)
	if err != nil {
		return err
	}
	if response.Status == statusError {
		return E.New("remote error: ", response.Message)
	}
	return nil
}

func (c *ClientPacketAddrConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	if !c.responseRead {
		err = c.readResponse()
		if err != nil {
			return
		}
		c.responseRead = true
	}
	destination, err := M.SocksaddrSerializer.ReadAddrPort(c.ExtendedConn)
	if err != nil {
		return
	}
	addr = destination.UDPAddr()
	var length uint16
	err = binary.Read(c.ExtendedConn, binary.BigEndian, &length)
	if err != nil {
		return
	}
	if cap(p) < int(length) {
		return 0, nil, io.ErrShortBuffer
	}
	n, err = io.ReadFull(c.ExtendedConn, p[:length])
	return
}

func (c *ClientPacketAddrConn) writeRequest(payload []byte, destination M.Socksaddr) (n int, err error) {
	request := StreamRequest{
		Network:     N.NetworkUDP,
		Destination: c.destination,
		PacketAddr:  true,
	}
	rLen := requestLen(request)
	if len(payload) > 0 {
		rLen += M.SocksaddrSerializer.AddrPortLen(destination) + 2 + len(payload)
	}
	_buffer := buf.StackNewSize(rLen)
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	EncodeStreamRequest(request, buffer)
	if len(payload) > 0 {
		common.Must(
			M.SocksaddrSerializer.WriteAddrPort(buffer, destination),
			binary.Write(buffer, binary.BigEndian, uint16(len(payload))),
			common.Error(buffer.Write(payload)),
		)
	}
	_, err = c.ExtendedConn.Write(buffer.Bytes())
	if err != nil {
		return
	}
	c.requestWrite = true
	return len(payload), nil
}

func (c *ClientPacketAddrConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	if !c.requestWrite {
		return c.writeRequest(p, M.SocksaddrFromNet(addr))
	}
	err = M.SocksaddrSerializer.WriteAddrPort(c.ExtendedConn, M.SocksaddrFromNet(addr))
	if err != nil {
		return
	}
	err = binary.Write(c.ExtendedConn, binary.BigEndian, uint16(len(p)))
	if err != nil {
		return
	}
	return c.ExtendedConn.Write(p)
}

func (c *ClientPacketAddrConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	if !c.responseRead {
		err = c.readResponse()
		if err != nil {
			return
		}
		c.responseRead = true
	}
	destination, err = M.SocksaddrSerializer.ReadAddrPort(c.ExtendedConn)
	if err != nil {
		return
	}
	var length uint16
	err = binary.Read(c.ExtendedConn, binary.BigEndian, &length)
	if err != nil {
		return
	}
	_, err = buffer.ReadFullFrom(c.ExtendedConn, int(length))
	return
}

func (c *ClientPacketAddrConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	if !c.requestWrite {
		defer buffer.Release()
		return common.Error(c.writeRequest(buffer.Bytes(), destination))
	}
	bLen := buffer.Len()
	header := buf.With(buffer.ExtendHeader(M.SocksaddrSerializer.AddrPortLen(destination) + 2))
	common.Must(
		M.SocksaddrSerializer.WriteAddrPort(header, destination),
		binary.Write(header, binary.BigEndian, uint16(bLen)),
	)
	return c.ExtendedConn.WriteBuffer(buffer)
}

func (c *ClientPacketAddrConn) LocalAddr() net.Addr {
	return c.ExtendedConn.LocalAddr()
}

func (c *ClientPacketAddrConn) FrontHeadroom() int {
	return 2 + M.MaxSocksaddrLength
}

func (c *ClientPacketAddrConn) Upstream() any {
	return c.ExtendedConn
}
