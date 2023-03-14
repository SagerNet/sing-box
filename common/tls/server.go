package tls

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/jobberrt/sing-box/common/badtls"
	C "github.com/jobberrt/sing-box/constant"
	"github.com/jobberrt/sing-box/log"
	"github.com/jobberrt/sing-box/option"
)

func NewServer(ctx context.Context, logger log.Logger, options option.InboundTLSOptions) (ServerConfig, error) {
	if !options.Enabled {
		return nil, nil
	}
	return NewSTDServer(ctx, logger, options)
}

func ServerHandshake(ctx context.Context, conn net.Conn, config ServerConfig) (Conn, error) {
	tlsConn := config.Server(conn)
	ctx, cancel := context.WithTimeout(ctx, C.TCPTimeout)
	defer cancel()
	err := tlsConn.HandshakeContext(ctx)
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
