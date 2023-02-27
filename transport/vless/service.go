package vless

import (
	"context"
	"encoding/binary"
	"io"
	"net"

	"github.com/sagernet/sing-vmess"
	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/gofrs/uuid"
)

type Service[T any] struct {
	userMap map[[16]byte]T
	logger  logger.Logger
	handler Handler
}

type Handler interface {
	N.TCPConnectionHandler
	N.UDPConnectionHandler
	E.Handler
}

func NewService[T any](logger logger.Logger, handler Handler) *Service[T] {
	return &Service[T]{
		logger:  logger,
		handler: handler,
	}
}

func (s *Service[T]) UpdateUsers(userList []T, userUUIDList []string) {
	userMap := make(map[[16]byte]T)
	for i, userName := range userList {
		userID := uuid.FromStringOrNil(userUUIDList[i])
		if userID == uuid.Nil {
			userID = uuid.NewV5(uuid.Nil, userUUIDList[i])
		}
		userMap[userID] = userName
	}
	s.userMap = userMap
}

var _ N.TCPConnectionHandler = (*Service[int])(nil)

func (s *Service[T]) NewConnection(ctx context.Context, conn net.Conn, metadata M.Metadata) error {
	request, err := ReadRequest(conn)
	if err != nil {
		return err
	}
	user, loaded := s.userMap[request.UUID]
	if !loaded {
		return E.New("unknown UUID: ", uuid.FromBytesOrNil(request.UUID[:]))
	}
	ctx = auth.ContextWithUser(ctx, user)
	metadata.Destination = request.Destination

	protocolConn := conn
	switch request.Flow {
	case "":
	case FlowVision:
		protocolConn, err = NewVisionConn(conn, request.UUID, s.logger)
		if err != nil {
			return E.Cause(err, "initialize vision")
		}
	}

	switch request.Command {
	case vmess.CommandTCP:
		return s.handler.NewConnection(ctx, &serverConn{Conn: protocolConn, responseWriter: conn}, metadata)
	case vmess.CommandUDP:
		return s.handler.NewPacketConnection(ctx, &serverPacketConn{ExtendedConn: bufio.NewExtendedConn(conn), destination: request.Destination}, metadata)
	case vmess.CommandMux:
		return vmess.HandleMuxConnection(ctx, &serverConn{Conn: conn}, s.handler)
	default:
		return E.New("unknown command: ", request.Command)
	}
}

type serverConn struct {
	net.Conn
	responseWriter  io.Writer
	responseWritten bool
}

func (c *serverConn) Read(b []byte) (n int, err error) {
	return c.Conn.Read(b)
}

func (c *serverConn) Write(b []byte) (n int, err error) {
	if !c.responseWritten {
		if c.responseWriter == nil {
			_, err = bufio.WriteVectorised(bufio.NewVectorisedWriter(c.Conn), [][]byte{{Version, 0}, b})
			if err == nil {
				n = len(b)
			}
			c.responseWritten = true
			return
		} else {
			_, err = c.responseWriter.Write([]byte{Version, 0})
			if err != nil {
				return
			}
			c.responseWritten = true
		}
	}
	return c.Conn.Write(b)
}

type serverPacketConn struct {
	N.ExtendedConn
	responseWriter  io.Writer
	responseWritten bool
	destination     M.Socksaddr
}

func (c *serverPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	n, err = c.ExtendedConn.Read(p)
	if err != nil {
		return
	}
	addr = c.destination.UDPAddr()
	return
}

func (c *serverPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	if !c.responseWritten {
		if c.responseWriter == nil {
			var packetLen [2]byte
			binary.BigEndian.PutUint16(packetLen[:], uint16(len(p)))
			_, err = bufio.WriteVectorised(bufio.NewVectorisedWriter(c.ExtendedConn), [][]byte{{Version, 0}, packetLen[:], p})
			if err == nil {
				n = len(p)
			}
			c.responseWritten = true
			return
		} else {
			_, err = c.responseWriter.Write([]byte{Version, 0})
			if err != nil {
				return
			}
			c.responseWritten = true
		}
	}
	return c.ExtendedConn.Write(p)
}

func (c *serverPacketConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	var packetLen uint16
	err = binary.Read(c.ExtendedConn, binary.BigEndian, &packetLen)
	if err != nil {
		return
	}

	_, err = buffer.ReadFullFrom(c.ExtendedConn, int(packetLen))
	if err != nil {
		return
	}

	destination = c.destination
	return
}

func (c *serverPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	if !c.responseWritten {
		if c.responseWriter == nil {
			var packetLen [2]byte
			binary.BigEndian.PutUint16(packetLen[:], uint16(buffer.Len()))
			err := bufio.NewVectorisedWriter(c.ExtendedConn).WriteVectorised([]*buf.Buffer{buf.As([]byte{Version, 0}), buf.As(packetLen[:]), buffer})
			c.responseWritten = true
			return err
		} else {
			_, err := c.responseWriter.Write([]byte{Version, 0})
			if err != nil {
				return err
			}
			c.responseWritten = true
		}
	}
	packetLen := buffer.Len()
	binary.BigEndian.PutUint16(buffer.ExtendHeader(2), uint16(packetLen))
	return c.ExtendedConn.WriteBuffer(buffer)
}

func (c *serverPacketConn) FrontHeadroom() int {
	return 2
}

func (c *serverPacketConn) Upstream() any {
	return c.ExtendedConn
}
