//go:build darwin && cgo

package tls

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/certificate"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/logger"
)

const appleTLSEngineName = "Apple TLS engine"

type appleClientConfig struct {
	systemTLSConfig
	userPEM []byte
}

func (c *appleClientConfig) Clone() Config {
	return &appleClientConfig{
		systemTLSConfig: c.systemTLSConfig.clone(),
		userPEM:         append([]byte(nil), c.userPEM...),
	}
}

func (c *appleClientConfig) resolveAnchors() (adapter.AppleAnchors, error) {
	if len(c.userPEM) > 0 {
		return certificate.NewAppleAnchors(c.userPEM)
	}
	return certificate.AcquireAnchors(nil, c.store), nil
}

func newAppleClient(ctx context.Context, logger logger.ContextLogger, serverAddress string, options option.OutboundTLSOptions, allowEmptyServerName bool) (Config, error) {
	base, validated, err := newSystemTLSConfig(ctx, serverAddress, options, allowEmptyServerName, appleTLSEngineName)
	if err != nil {
		return nil, err
	}
	return &appleClientConfig{
		systemTLSConfig: base,
		userPEM:         append([]byte(nil), validated.UserPEM...),
	}, nil
}
