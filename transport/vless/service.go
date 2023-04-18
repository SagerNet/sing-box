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

	"github.com/gofrs/uuid/v5"
)

type Service[T comparable] struct {
	userMap  map[[16]byte]T
	userFlow map[T]string
	logger   logger.Logger
	handler  Handler
}

type Handler interface {
	N.TCPConnectionHandler
	N.UDPConnectionHandler
	E.Handler
}

func NewService[T comparable](logger logger.Logger, handler Handler) *Service[T] {
	return &Service[T]{
		logger:  logger,
		handler: handler,
	}
}

func (s *Service[T]) UpdateUsers(userList []T, userUUIDList []string, userFlowList []string) {
	userMap := make(map[[16]byte]T)
	userFlowMap := make(map[T]string)
	for i, userName := range userList {
		userID := uuid.FromStringOrNil(userUUIDList[i])
		if userID == uuid.Nil {
			userID = uuid.NewV5(uuid.Nil, userUUIDList[i])
		}
		userMap[userID] = userName
		userFlowMap[userName] = userFlowList[i]
	}
	s.userMap = userMap
	s.userFlow = userFlowMap
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

	userFlow := s.userFlow[user]
	if request.Flow == FlowVision && request.Command == vmess.NetworkUDP {
		return E.New(FlowVision, " flow does not support UDP")
	} else if request.Flow != userFlow {
		return E.New("flow mismatch: expected ", flowName(userFlow), ", but got ", flowName(request.Flow))
	}

	if request.Command == vmess.CommandUDP {
		return s.handler.NewPacketConnection(ctx, &serverPacketConn{ExtendedConn: bufio.NewExtendedConn(conn), destination: request.Destination}, metadata)
	}
	responseConn := &serverConn{ExtendedConn: bufio.NewExtendedConn(conn), writer: bufio.NewVectorisedWriter(conn)}
	switch userFlow {
	case FlowVision:
		conn, err = NewVisionConn(responseConn, conn, request.UUID, s.logger)
		if err != nil {
			return E.Cause(err, "initialize vision")
		}
	case "":
		conn = responseConn
	default:
		return E.New("unknown flow: ", userFlow)
	}
	switch request.Command {
	case vmess.CommandTCP:
		return s.handler.NewConnection(ctx, conn, metadata)
	case vmess.CommandMux:
		return vmess.HandleMuxConnection(ctx, conn, s.handler)
	default:
		return E.New("unknown command: ", request.Command)
	}
}

func flowName(value string) string {
	if value == "" {
		return "none"
	}
	return value
}

var _ N.VectorisedWriter = (*serverConn)(nil)

type serverConn struct {
	N.ExtendedConn
	writer          N.VectorisedWriter
	responseWritten bool
}

func (c *serverConn) Read(b []byte) (n int, err error) {
	return c.ExtendedConn.Read(b)
}

func (c *serverConn) Write(b []byte) (n int, err error) {
	if !c.responseWritten {
		_, err = bufio.WriteVectorised(c.writer, [][]byte{{Version, 0}, b})
		if err == nil {
			n = len(b)
		}
		c.responseWritten = true
		return
	}
	return c.ExtendedConn.Write(b)
}

func (c *serverConn) WriteBuffer(buffer *buf.Buffer) error {
	if !c.responseWritten {
		header := buffer.ExtendHeader(2)
		header[0] = Version
		header[1] = 0
		c.responseWritten = true
	}
	return c.ExtendedConn.WriteBuffer(buffer)
}

func (c *serverConn) WriteVectorised(buffers []*buf.Buffer) error {
	if !c.responseWritten {
		err := c.writer.WriteVectorised(append([]*buf.Buffer{buf.As([]byte{Version, 0})}, buffers...))
		c.responseWritten = true
		return err
	}
	return c.writer.WriteVectorised(buffers)
}

func (c *serverConn) NeedAdditionalReadDeadline() bool {
	return true
}

func (c *serverConn) FrontHeadroom() int {
	if c.responseWritten {
		return 0
	}
	return 2
}

func (c *serverConn) ReaderReplaceable() bool {
	return true
}

func (c *serverConn) WriterReplaceable() bool {
	return c.responseWritten
}

func (c *serverConn) Upstream() any {
	return c.ExtendedConn
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
	if c.destination.IsFqdn() {
		addr = c.destination
	} else {
		addr = c.destination.UDPAddr()
	}
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
