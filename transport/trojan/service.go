package trojan

import (
	"context"
	"encoding/binary"
	"net"

	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/rw"
)

type Handler interface {
	N.TCPConnectionHandlerEx
	N.UDPConnectionHandlerEx
}

type Service[K comparable] struct {
	users           map[K][56]byte
	keys            map[[56]byte]K
	handler         Handler
	fallbackHandler N.TCPConnectionHandlerEx
	logger          logger.ContextLogger
}

func NewService[K comparable](handler Handler, fallbackHandler N.TCPConnectionHandlerEx, logger logger.ContextLogger) *Service[K] {
	return &Service[K]{
		users:           make(map[K][56]byte),
		keys:            make(map[[56]byte]K),
		handler:         handler,
		fallbackHandler: fallbackHandler,
		logger:          logger,
	}
}

var ErrUserExists = E.New("user already exists")

func (s *Service[K]) UpdateUsers(userList []K, passwordList []string) error {
	users := make(map[K][56]byte)
	keys := make(map[[56]byte]K)
	for i, user := range userList {
		if _, loaded := users[user]; loaded {
			return ErrUserExists
		}
		key := Key(passwordList[i])
		if oldUser, loaded := keys[key]; loaded {
			return E.Extend(ErrUserExists, "password used by ", oldUser)
		}
		users[user] = key
		keys[key] = user
	}
	s.users = users
	s.keys = keys
	return nil
}

func (s *Service[K]) NewConnection(ctx context.Context, conn net.Conn, source M.Socksaddr, onClose N.CloseHandlerFunc) error {
	var key [KeyLength]byte
	n, err := conn.Read(key[:])
	if err != nil {
		return err
	} else if n != KeyLength {
		return s.fallback(ctx, conn, source, key[:n], E.New("bad request size"), onClose)
	}

	if user, loaded := s.keys[key]; loaded {
		ctx = auth.ContextWithUser(ctx, user)
	} else {
		return s.fallback(ctx, conn, source, key[:], E.New("bad request"), onClose)
	}

	err = rw.SkipN(conn, 2)
	if err != nil {
		return E.Cause(err, "skip crlf")
	}

	var command byte
	err = binary.Read(conn, binary.BigEndian, &command)
	if err != nil {
		return E.Cause(err, "read command")
	}

	switch command {
	case CommandTCP, CommandUDP, CommandMux:
	default:
		return E.New("unknown command ", command)
	}

	// var destination M.Socksaddr
	destination, err := M.SocksaddrSerializer.ReadAddrPort(conn)
	if err != nil {
		return E.Cause(err, "read destination")
	}

	err = rw.SkipN(conn, 2)
	if err != nil {
		return E.Cause(err, "skip crlf")
	}

	switch command {
	case CommandTCP:
		s.handler.NewConnectionEx(ctx, conn, source, destination, onClose)
	case CommandUDP:
		s.handler.NewPacketConnectionEx(ctx, &PacketConn{Conn: conn}, source, destination, onClose)
	// case CommandMux:
	default:
		return HandleMuxConnection(ctx, conn, source, s.handler, s.logger, onClose)
	}
	return nil
}

func (s *Service[K]) fallback(ctx context.Context, conn net.Conn, source M.Socksaddr, header []byte, err error, onClose N.CloseHandlerFunc) error {
	if s.fallbackHandler == nil {
		return E.Extend(err, "fallback disabled")
	}
	conn = bufio.NewCachedConn(conn, buf.As(header).ToOwned())
	s.fallbackHandler.NewConnectionEx(ctx, conn, source, M.Socksaddr{}, onClose)
	return nil
}

type PacketConn struct {
	net.Conn
	readWaitOptions N.ReadWaitOptions
}

func (c *PacketConn) ReadPacket(buffer *buf.Buffer) (M.Socksaddr, error) {
	return ReadPacket(c.Conn, buffer)
}

func (c *PacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	return WritePacket(c.Conn, buffer, destination)
}

func (c *PacketConn) FrontHeadroom() int {
	return M.MaxSocksaddrLength + 4
}

func (c *PacketConn) NeedAdditionalReadDeadline() bool {
	return true
}

func (c *PacketConn) Upstream() any {
	return c.Conn
}
