package mux

import (
	"context"
	"encoding/binary"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/rw"
)

func NewConnection(ctx context.Context, router adapter.Router, errorHandler E.Handler, logger log.ContextLogger, conn net.Conn, metadata adapter.InboundContext) error {
	request, err := ReadRequest(conn)
	if err != nil {
		return err
	}
	session, err := request.Protocol.newServer(conn)
	if err != nil {
		return err
	}
	var stream net.Conn
	for {
		stream, err = session.Accept()
		if err != nil {
			return err
		}
		go newConnection(ctx, router, errorHandler, logger, stream, metadata)
	}
}

func newConnection(ctx context.Context, router adapter.Router, errorHandler E.Handler, logger log.ContextLogger, stream net.Conn, metadata adapter.InboundContext) {
	stream = &wrapStream{stream}
	request, err := ReadStreamRequest(stream)
	if err != nil {
		logger.ErrorContext(ctx, err)
		return
	}
	metadata.Destination = request.Destination
	if request.Network == N.NetworkTCP {
		logger.InfoContext(ctx, "inbound multiplex connection to ", metadata.Destination)
		hErr := router.RouteConnection(ctx, &ServerConn{ExtendedConn: bufio.NewExtendedConn(stream)}, metadata)
		stream.Close()
		if hErr != nil {
			errorHandler.NewError(ctx, hErr)
		}
	} else {
		var packetConn N.PacketConn
		if !request.PacketAddr {
			logger.InfoContext(ctx, "inbound multiplex packet connection to ", metadata.Destination)
			packetConn = &ServerPacketConn{ExtendedConn: bufio.NewExtendedConn(stream), destination: request.Destination}
		} else {
			logger.InfoContext(ctx, "inbound multiplex packet connection")
			packetConn = &ServerPacketAddrConn{ExtendedConn: bufio.NewExtendedConn(stream)}
		}
		hErr := router.RoutePacketConnection(ctx, packetConn, metadata)
		stream.Close()
		if hErr != nil {
			errorHandler.NewError(ctx, hErr)
		}
	}
}

var _ N.HandshakeConn = (*ServerConn)(nil)

type ServerConn struct {
	N.ExtendedConn
	responseWrite bool
}

func (c *ServerConn) HandshakeFailure(err error) error {
	errMessage := err.Error()
	_buffer := buf.StackNewSize(1 + rw.UVariantLen(uint64(len(errMessage))) + len(errMessage))
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	common.Must(
		buffer.WriteByte(statusError),
		rw.WriteVString(_buffer, errMessage),
	)
	return c.ExtendedConn.WriteBuffer(buffer)
}

func (c *ServerConn) Write(b []byte) (n int, err error) {
	if c.responseWrite {
		return c.ExtendedConn.Write(b)
	}
	_buffer := buf.StackNewSize(1 + len(b))
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	common.Must(
		buffer.WriteByte(statusSuccess),
		common.Error(buffer.Write(b)),
	)
	_, err = c.ExtendedConn.Write(buffer.Bytes())
	if err != nil {
		return
	}
	c.responseWrite = true
	return len(b), nil
}

func (c *ServerConn) WriteBuffer(buffer *buf.Buffer) error {
	if c.responseWrite {
		return c.ExtendedConn.WriteBuffer(buffer)
	}
	buffer.ExtendHeader(1)[0] = statusSuccess
	c.responseWrite = true
	return c.ExtendedConn.WriteBuffer(buffer)
}

func (c *ServerConn) FrontHeadroom() int {
	if !c.responseWrite {
		return 1
	}
	return 0
}

func (c *ServerConn) Upstream() any {
	return c.ExtendedConn
}

var (
	_ N.HandshakeConn = (*ServerPacketConn)(nil)
	_ N.PacketConn    = (*ServerPacketConn)(nil)
)

type ServerPacketConn struct {
	N.ExtendedConn
	destination   M.Socksaddr
	responseWrite bool
}

func (c *ServerPacketConn) HandshakeFailure(err error) error {
	errMessage := err.Error()
	_buffer := buf.StackNewSize(1 + rw.UVariantLen(uint64(len(errMessage))) + len(errMessage))
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	common.Must(
		buffer.WriteByte(statusError),
		rw.WriteVString(_buffer, errMessage),
	)
	return c.ExtendedConn.WriteBuffer(buffer)
}

func (c *ServerPacketConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	var length uint16
	err = binary.Read(c.ExtendedConn, binary.BigEndian, &length)
	if err != nil {
		return
	}
	_, err = buffer.ReadFullFrom(c.ExtendedConn, int(length))
	if err != nil {
		return
	}
	destination = c.destination
	return
}

func (c *ServerPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	pLen := buffer.Len()
	common.Must(binary.Write(buf.With(buffer.ExtendHeader(2)), binary.BigEndian, uint16(pLen)))
	if !c.responseWrite {
		buffer.ExtendHeader(1)[0] = statusSuccess
		c.responseWrite = true
	}
	return c.ExtendedConn.WriteBuffer(buffer)
}

func (c *ServerPacketConn) Upstream() any {
	return c.ExtendedConn
}

func (c *ServerPacketConn) FrontHeadroom() int {
	if !c.responseWrite {
		return 3
	}
	return 2
}

var (
	_ N.HandshakeConn = (*ServerPacketAddrConn)(nil)
	_ N.PacketConn    = (*ServerPacketAddrConn)(nil)
)

type ServerPacketAddrConn struct {
	N.ExtendedConn
	responseWrite bool
}

func (c *ServerPacketAddrConn) HandshakeFailure(err error) error {
	errMessage := err.Error()
	_buffer := buf.StackNewSize(1 + rw.UVariantLen(uint64(len(errMessage))) + len(errMessage))
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	common.Must(
		buffer.WriteByte(statusError),
		rw.WriteVString(_buffer, errMessage),
	)
	return c.ExtendedConn.WriteBuffer(buffer)
}

func (c *ServerPacketAddrConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
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
	if err != nil {
		return
	}
	return
}

func (c *ServerPacketAddrConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	pLen := buffer.Len()
	common.Must(binary.Write(buf.With(buffer.ExtendHeader(2)), binary.BigEndian, uint16(pLen)))
	common.Must(M.SocksaddrSerializer.WriteAddrPort(buf.With(buffer.ExtendHeader(M.SocksaddrSerializer.AddrPortLen(destination))), destination))
	if !c.responseWrite {
		buffer.ExtendHeader(1)[0] = statusSuccess
		c.responseWrite = true
	}
	return c.ExtendedConn.WriteBuffer(buffer)
}

func (c *ServerPacketAddrConn) Upstream() any {
	return c.ExtendedConn
}

func (c *ServerPacketAddrConn) FrontHeadroom() int {
	if !c.responseWrite {
		return 3 + M.MaxSocksaddrLength
	}
	return 2 + M.MaxSocksaddrLength
}
