package limiter

import (
	"context"
	"net"
)

type Manager interface {
	LoadLimiters(tags []string, user, inbound string) []*limiter
	NewConnWithLimiters(ctx context.Context, conn net.Conn, limiters []*limiter) net.Conn
}
