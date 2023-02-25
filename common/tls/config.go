package tls

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
)

type (
	STDConfig       = tls.Config
	STDConn         = tls.Conn
	ConnectionState = tls.ConnectionState
)

type Config interface {
	ServerName() string
	SetServerName(serverName string)
	NextProtos() []string
	SetNextProtos(nextProto []string)
	Config() (*STDConfig, error)
	Client(conn net.Conn) (Conn, error)
	Clone() Config
}

type ConfigWithSessionIDGenerator interface {
	SetSessionIDGenerator(generator func(clientHello []byte, sessionID []byte) error)
}

type ServerConfig interface {
	Config
	adapter.Service
	Server(conn net.Conn) (Conn, error)
}

type ServerConfigCompat interface {
	ServerConfig
	ServerHandshake(ctx context.Context, conn net.Conn) (Conn, error)
}

type Conn interface {
	net.Conn
	HandshakeContext(ctx context.Context) error
	ConnectionState() ConnectionState
}

func ParseTLSVersion(version string) (uint16, error) {
	switch version {
	case "1.0":
		return tls.VersionTLS10, nil
	case "1.1":
		return tls.VersionTLS11, nil
	case "1.2":
		return tls.VersionTLS12, nil
	case "1.3":
		return tls.VersionTLS13, nil
	default:
		return 0, E.New("unknown tls version:", version)
	}
}
