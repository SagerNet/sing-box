package tls

import (
	"context"
	"net"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	aTLS "github.com/sagernet/sing/common/tls"
)

func NewServer(ctx context.Context, logger log.Logger, options option.InboundTLSOptions) (ServerConfig, error) {
	if !options.Enabled {
		return nil, nil
	}
	if options.ECH != nil && options.ECH.Enabled {
		return NewECHServer(ctx, logger, options)
	} else if options.Reality != nil && options.Reality.Enabled {
		return NewRealityServer(ctx, logger, options)
	} else {
		return NewSTDServer(ctx, logger, options)
	}
}

func ServerHandshake(ctx context.Context, conn net.Conn, config ServerConfig) (Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, C.TCPTimeout)
	defer cancel()
	return aTLS.ServerHandshake(ctx, conn, config)
}
