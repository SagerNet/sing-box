//go:build !with_reality_server

package tls

import (
	"context"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func NewRealityServer(ctx context.Context, logger log.Logger, options option.InboundTLSOptions) (ServerConfig, error) {
	return nil, E.New(`reality server is not included in this build, rebuild with -tags with_reality_server`)
}
