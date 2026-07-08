//go:build !linux && !darwin && !(windows && (amd64 || 386))

package bridge

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
)

func newBackend(ctx context.Context, logger logger.ContextLogger, networkManager adapter.NetworkManager, tag string, options option.BridgeOutboundOptions) (Backend, error) {
	return nil, E.New("bridge outbound is only supported on Linux, macOS, Windows (x86 and x64), rooted Android and jailbroken iOS")
}
