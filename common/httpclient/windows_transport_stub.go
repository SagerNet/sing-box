//go:build !windows

package httpclient

import (
	"context"

	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	N "github.com/sagernet/sing/common/network"
)

func newWindowsTransport(ctx context.Context, logger logger.ContextLogger, rawDialer N.Dialer, options option.HTTPClientOptions) (innerTransport, error) {
	return nil, E.New("Windows HTTP engine is not available on non-Windows platforms")
}
