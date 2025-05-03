//go:build !with_utls

package tls

import (
	"context"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func NewUTLSClient(ctx context.Context, serverAddress string, options option.OutboundTLSOptions) (Config, error) {
	return nil, E.New(`uTLS is not included in this build, rebuild with -tags with_utls`)
}

func NewRealityClient(ctx context.Context, serverAddress string, options option.OutboundTLSOptions) (Config, error) {
	return nil, E.New(`uTLS, which is required by reality is not included in this build, rebuild with -tags with_utls`)
}

func NewRealityServer(ctx context.Context, logger log.Logger, options option.InboundTLSOptions) (ServerConfig, error) {
	return nil, E.New(`uTLS, which is required by reality is not included in this build, rebuild with -tags with_utls`)
}
