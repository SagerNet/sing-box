//go:build with_quic && with_ech

package tls

import (
	"context"
	"net"
	"net/http"

	"github.com/sagernet/cloudflare-tls"
	"github.com/sagernet/quic-go/ech"
	"github.com/sagernet/quic-go/http3_ech"
	"github.com/sagernet/sing-quic"
	M "github.com/sagernet/sing/common/metadata"
)

var (
	_ qtls.Config       = (*echClientConfig)(nil)
	_ qtls.ServerConfig = (*echServerConfig)(nil)
)

func (c *echClientConfig) Dial(ctx context.Context, conn net.PacketConn, addr net.Addr, config *quic.Config) (quic.Connection, error) {
	return quic.Dial(ctx, conn, addr, c.config, config)
}

func (c *echClientConfig) DialEarly(ctx context.Context, conn net.PacketConn, addr net.Addr, config *quic.Config) (quic.EarlyConnection, error) {
	return quic.DialEarly(ctx, conn, addr, c.config, config)
}

func (c *echClientConfig) CreateTransport(conn net.PacketConn, quicConnPtr *quic.EarlyConnection, serverAddr M.Socksaddr, quicConfig *quic.Config) http.RoundTripper {
	return &http3.RoundTripper{
		TLSClientConfig: c.config,
		QUICConfig:      quicConfig,
		Dial: func(ctx context.Context, addr string, tlsCfg *tls.Config, cfg *quic.Config) (quic.EarlyConnection, error) {
			quicConn, err := quic.DialEarly(ctx, conn, serverAddr.UDPAddr(), tlsCfg, cfg)
			if err != nil {
				return nil, err
			}
			*quicConnPtr = quicConn
			return quicConn, nil
		},
	}
}

func (c *echServerConfig) Listen(conn net.PacketConn, config *quic.Config) (qtls.Listener, error) {
	return quic.Listen(conn, c.config, config)
}

func (c *echServerConfig) ListenEarly(conn net.PacketConn, config *quic.Config) (qtls.EarlyListener, error) {
	return quic.ListenEarly(conn, c.config, config)
}

func (c *echServerConfig) ConfigureHTTP3() {
	http3.ConfigureTLSConfig(c.config)
}
