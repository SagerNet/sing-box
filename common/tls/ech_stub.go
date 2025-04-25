//go:build !go1.24

package tls

import (
	"context"
	"crypto/tls"

	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func parseECHClientConfig(ctx context.Context, options option.OutboundTLSOptions, tlsConfig *tls.Config) (Config, error) {
	return nil, E.New("ECH requires go1.24, please recompile your binary.")
}

func parseECHServerConfig(ctx context.Context, options option.InboundTLSOptions, tlsConfig *tls.Config, echKeyPath *string) error {
	return E.New("ECH requires go1.24, please recompile your binary.")
}

func reloadECHKeys(echKeyPath string, tlsConfig *tls.Config) error {
	return E.New("ECH requires go1.24, please recompile your binary.")
}
