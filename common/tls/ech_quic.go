//go:build with_quic && with_ech

package tls

import (
	"context"
	"net"

	"github.com/sagernet/quic-go/ech"
	"github.com/sagernet/sing-box/common/qtls"
)

var (
	_ qtls.QUICConfig       = (*echClientConfig)(nil)
	_ qtls.QUICServerConfig = (*echServerConfig)(nil)
)

func (c *echClientConfig) Dial(ctx context.Context, conn net.PacketConn, addr net.Addr, config *quic.Config) (quic.Connection, error) {
	return quic.Dial(ctx, conn, addr, c.config, config)
}

func (c *echClientConfig) DialEarly(ctx context.Context, conn net.PacketConn, addr net.Addr, config *quic.Config) (quic.EarlyConnection, error) {
	return quic.DialEarly(ctx, conn, addr, c.config, config)
}

func (c *echServerConfig) Listen(conn net.PacketConn, config *quic.Config) (qtls.QUICListener, error) {
	return quic.Listen(conn, c.config, config)
}

func (c *echServerConfig) ListenEarly(conn net.PacketConn, config *quic.Config) (qtls.QUICEarlyListener, error) {
	return quic.ListenEarly(conn, c.config, config)
}
