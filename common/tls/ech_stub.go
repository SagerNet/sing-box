//go:build !with_ech

package tls

import (
	"context"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func NewECHServer(ctx context.Context, logger log.Logger, options option.InboundTLSOptions) (ServerConfig, error) {
	return nil, E.New(`ECH is not included in this build, rebuild with -tags with_ech`)
}

func NewECHClient(ctx context.Context, serverAddress string, options option.OutboundTLSOptions) (Config, error) {
	return nil, E.New(`ECH is not included in this build, rebuild with -tags with_ech`)
}
