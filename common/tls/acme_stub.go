//go:build !with_acme

package tls

import (
	"context"

	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
)

func startACME(ctx context.Context, logger logger.Logger, options option.InboundACMEOptions) (managedCertificateProvider, error) {
	return nil, E.New(`ACME is not included in this build, rebuild with -tags with_acme`)
}
