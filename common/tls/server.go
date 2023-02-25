package tls

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/badtls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

func NewServer(ctx context.Context, router adapter.Router, logger log.Logger, options option.InboundTLSOptions) (ServerConfig, error) {
	if !options.Enabled {
		return nil, nil
	}
	if options.Reality != nil && options.Reality.Enabled {
		return NewRealityServer(ctx, router, logger, options)
	} else {
		return NewSTDServer(ctx, router, logger, options)
	}
}

func ServerHandshake(ctx context.Context, conn net.Conn, config ServerConfig) (Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, C.TCPTimeout)
	defer cancel()
	if compatServer, isCompat := config.(ServerConfigCompat); isCompat {
		return compatServer.ServerHandshake(ctx, conn)
	}
	tlsConn, err := config.Server(conn)
	if err != nil {
		return nil, err
	}
	err = tlsConn.HandshakeContext(ctx)
	if err != nil {
		return nil, err
	}
	if stdConn, isSTD := tlsConn.(*tls.Conn); isSTD {
		var badConn badtls.TLSConn
		badConn, err = badtls.Create(stdConn)
		if err == nil {
			return badConn, nil
		}
	}
	return tlsConn, nil
}
