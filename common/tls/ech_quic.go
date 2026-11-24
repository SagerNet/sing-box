//go:build go1.24 && with_quic

package tls

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/http3"
	qtls "github.com/sagernet/sing-quic"
)

func (s *ECHClientConfig) fetchSTDConfig(ctx context.Context) (*STDConfig, error) {
	tlsConfig, err := s.handshakeConfig(ctx)
	if err != nil {
		return nil, err
	}
	return tlsConfig.STDConfig()
}

func (s *ECHClientConfig) Dial(ctx context.Context, conn net.Conn, config *quic.Config) (*quic.Conn, error) {
	tlsConfig, err := s.fetchSTDConfig(ctx)
	if err != nil {
		return nil, err
	}
	return quic.DialConn(ctx, conn, tlsConfig, config)
}

func (s *ECHClientConfig) DialEarly(ctx context.Context, conn net.Conn, config *quic.Config) (*quic.Conn, error) {
	tlsConfig, err := s.fetchSTDConfig(ctx)
	if err != nil {
		return nil, err
	}
	return quic.DialEarlyConn(ctx, conn, tlsConfig, config)
}

func (s *ECHClientConfig) CreateTransport(conn net.Conn, quicConnPtr **quic.Conn, quicConfig *quic.Config) http.RoundTripper {
	return s.createHTTP3Transport(quicConnPtr, quicConfig, func(ctx context.Context, tlsConfig *tls.Config, config *quic.Config) (*quic.Conn, error) {
		return quic.DialEarlyConn(ctx, conn, tlsConfig, config)
	})
}

func (s *ECHClientConfig) CreatePacketTransport(conn net.PacketConn, remoteAddr net.Addr, quicConnPtr **quic.Conn, quicConfig *quic.Config) http.RoundTripper {
	return s.createHTTP3Transport(quicConnPtr, quicConfig, func(ctx context.Context, tlsConfig *tls.Config, config *quic.Config) (*quic.Conn, error) {
		return quic.DialEarly(ctx, conn, remoteAddr, tlsConfig, config)
	})
}

func (s *ECHClientConfig) createHTTP3Transport(quicConnPtr **quic.Conn, quicConfig *quic.Config, dial func(ctx context.Context, tlsConfig *tls.Config, config *quic.Config) (*quic.Conn, error)) http.RoundTripper {
	return &http3.Transport{
		QUICConfig: quicConfig,
		Dial: func(ctx context.Context, _ string, _ *tls.Config, config *quic.Config) (*quic.Conn, error) {
			tlsConfig, err := s.fetchSTDConfig(ctx)
			if err != nil {
				return nil, err
			}
			tlsConfig.NextProtos = []string{http3.NextProtoH3}
			quicConn, err := dial(ctx, tlsConfig, config)
			if err != nil {
				return nil, qtls.WrapError(err)
			}
			*quicConnPtr = quicConn
			return quicConn, nil
		},
	}
}

var _ qtls.Config = (*ECHClientConfig)(nil)
