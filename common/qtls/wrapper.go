package qtls

import (
	"context"
	"net"

	"github.com/sagernet/quic-go"
	aTLS "github.com/sagernet/sing/common/tls"
)

type QUICConfig interface {
	Dial(ctx context.Context, conn net.PacketConn, addr net.Addr, config *quic.Config) (quic.Connection, error)
	DialEarly(ctx context.Context, conn net.PacketConn, addr net.Addr, config *quic.Config) (quic.EarlyConnection, error)
}

type QUICServerConfig interface {
	Listen(conn net.PacketConn, config *quic.Config) (QUICListener, error)
	ListenEarly(conn net.PacketConn, config *quic.Config) (QUICEarlyListener, error)
}

type QUICListener interface {
	Accept(ctx context.Context) (quic.Connection, error)
	Close() error
	Addr() net.Addr
}

type QUICEarlyListener interface {
	Accept(ctx context.Context) (quic.EarlyConnection, error)
	Close() error
	Addr() net.Addr
}

func Dial(ctx context.Context, conn net.PacketConn, addr net.Addr, config aTLS.Config, quicConfig *quic.Config) (quic.Connection, error) {
	if quicTLSConfig, isQUICConfig := config.(QUICConfig); isQUICConfig {
		return quicTLSConfig.Dial(ctx, conn, addr, quicConfig)
	}
	tlsConfig, err := config.Config()
	if err != nil {
		return nil, err
	}
	return quic.Dial(ctx, conn, addr, tlsConfig, quicConfig)
}

func DialEarly(ctx context.Context, conn net.PacketConn, addr net.Addr, config aTLS.Config, quicConfig *quic.Config) (quic.EarlyConnection, error) {
	if quicTLSConfig, isQUICConfig := config.(QUICConfig); isQUICConfig {
		return quicTLSConfig.DialEarly(ctx, conn, addr, quicConfig)
	}
	tlsConfig, err := config.Config()
	if err != nil {
		return nil, err
	}
	return quic.DialEarly(ctx, conn, addr, tlsConfig, quicConfig)
}

func Listen(conn net.PacketConn, config aTLS.ServerConfig, quicConfig *quic.Config) (QUICListener, error) {
	if quicTLSConfig, isQUICConfig := config.(QUICServerConfig); isQUICConfig {
		return quicTLSConfig.Listen(conn, quicConfig)
	}
	tlsConfig, err := config.Config()
	if err != nil {
		return nil, err
	}
	return quic.Listen(conn, tlsConfig, quicConfig)
}

func ListenEarly(conn net.PacketConn, config aTLS.ServerConfig, quicConfig *quic.Config) (QUICEarlyListener, error) {
	if quicTLSConfig, isQUICConfig := config.(QUICServerConfig); isQUICConfig {
		return quicTLSConfig.ListenEarly(conn, quicConfig)
	}
	tlsConfig, err := config.Config()
	if err != nil {
		return nil, err
	}
	return quic.ListenEarly(conn, tlsConfig, quicConfig)
}
