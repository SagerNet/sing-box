package limiter

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
)

type Manager interface {
	NewConnWithLimiters(ctx context.Context, conn net.Conn, metadata *adapter.InboundContext, rule adapter.Rule) net.Conn
}
