//go:build !with_acme

package tls

import (
	"context"
	"crypto/tls"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func startACME(ctx context.Context, options option.InboundACMEOptions) (*tls.Config, adapter.SimpleLifecycle, error) {
	return nil, nil, E.New(`ACME is not included in this build, rebuild with -tags with_acme`)
}
