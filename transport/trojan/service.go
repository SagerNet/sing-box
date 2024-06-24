package trojan

import (
	"context"
	"encoding/binary"
	"net"

	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/rw"
)

type Handler interface {
	N.TCPConnectionHandler
	N.UDPConnectionHandler
	E.Handler
}

type Service[K comparable] struct {
	users           map[K][56]byte
	keys            map[[56]byte]K
	handler         Handler
	fallbackHandler N.TCPConnectionHandler
}

func NewService[K comparable](handler Handler, fallbackHandler N.TCPConnectionHandler) *Service[K] {
	return &Service[K]{
		users:           make(map[K][56]byte),
		keys:            make(map[[56]byte]K),
		handler:         handler,
		fallbackHandler: fallbackHandler,
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

func (s *Service[K]) NewConnection(ctx context.Context, conn net.Conn, metadata M.Metadata) error {
	var key [KeyLength]byte
	n, err := conn.Read(key[:])
	if err != nil {
		return err
	} else if n != KeyLength {
		return s.fallback(ctx, conn, metadata, key[:n], E.New("bad request size"))
	}

	if user, loaded := s.keys[key]; loaded {
		ctx = auth.ContextWithUser(ctx, user)
	} else {
		return s.fallback(ctx, conn, metadata, key[:], E.New("bad request"))
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

	metadata.Protocol = "trojan"
	metadata.Destination = destination

	switch command {
	case CommandTCP:
		return s.handler.NewConnection(ctx, conn, metadata)
	case CommandUDP:
		return s.handler.NewPacketConnection(ctx, &PacketConn{Conn: conn}, metadata)
	// case CommandMux:
	default:
		return HandleMuxConnection(ctx, conn, metadata, s.handler)
	}
}

func (s *Service[K]) fallback(ctx context.Context, conn net.Conn, metadata M.Metadata, header []byte, err error) error {
	if s.fallbackHandler == nil {
		return E.Extend(err, "fallback disabled")
	}
	conn = bufio.NewCachedConn(conn, buf.As(header).ToOwned())
	return s.fallbackHandler.NewConnection(ctx, conn, metadata)
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
