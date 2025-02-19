package anytls

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"net"
	"os"

	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/transport/anytls/padding"
	"github.com/sagernet/sing-box/transport/anytls/session"
	"github.com/sagernet/sing/common/atomic"
	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type Service struct {
	users     map[[32]byte]string
	padding   atomic.TypedValue[*padding.PaddingFactory]
	tlsConfig tls.ServerConfig
	handler   N.TCPConnectionHandlerEx
	logger    logger.ContextLogger
}

type ServiceConfig struct {
	PaddingScheme []byte
	Users         []User
	TLSConfig     tls.ServerConfig
	Handler       N.TCPConnectionHandlerEx
	Logger        logger.ContextLogger
}

type User struct {
	Name     string
	Password string
}

func NewService(config ServiceConfig) (*Service, error) {
	service := &Service{
		tlsConfig: config.TLSConfig,
		handler:   config.Handler,
		logger:    config.Logger,
		users:     make(map[[32]byte]string),
	}

	if service.handler == nil || service.logger == nil {
		return nil, os.ErrInvalid
	}

	for _, user := range config.Users {
		service.users[sha256.Sum256([]byte(user.Password))] = user.Name
	}

	if !padding.UpdatePaddingScheme(config.PaddingScheme, &service.padding) {
		return nil, errors.New("incorrect padding scheme format")
	}

	return service, nil
}

func (s *Service) NewConnection(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) error {
	var err error

	if s.tlsConfig != nil {
		conn, err = tls.ServerHandshake(ctx, conn, s.tlsConfig)
		if err != nil {
			return err
		}
	}

	b := buf.NewPacket()
	defer b.Release()

	_, err = b.ReadOnceFrom(conn)
	if err != nil {
		return err
	}
	conn = bufio.NewCachedConn(conn, b)

	by, err := b.ReadBytes(32)
	if err != nil {
		b.Reset()
		return os.ErrInvalid
	}
	var passwordSha256 [32]byte
	copy(passwordSha256[:], by)
	if user, ok := s.users[passwordSha256]; ok {
		ctx = auth.ContextWithUser(ctx, user)
	} else {
		b.Reset()
		return os.ErrInvalid
	}
	by, err = b.ReadBytes(2)
	if err != nil {
		b.Reset()
		return os.ErrInvalid
	}
	paddingLen := binary.BigEndian.Uint16(by)
	if paddingLen > 0 {
		_, err = b.ReadBytes(int(paddingLen))
		if err != nil {
			b.Reset()
			return os.ErrInvalid
		}
	}

	session := session.NewServerSession(conn, func(stream *session.Stream) {
		destination, err := M.SocksaddrSerializer.ReadAddrPort(stream)
		if err != nil {
			s.logger.ErrorContext(ctx, "ReadAddrPort:", err)
			return
		}

		s.handler.NewConnectionEx(ctx, stream, source, destination, onClose)
	}, &s.padding)
	session.Run()
	session.Close()
	return nil
}
