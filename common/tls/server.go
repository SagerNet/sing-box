package tls

import (
	"context"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

func NewServer(ctx context.Context, logger log.Logger, options option.InboundTLSOptions) (ServerConfig, error) {
	return newSTDServer(ctx, logger, options)
}
