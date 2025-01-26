//go:build !(linux || darwin)

package tf

import (
	"context"
	"syscall"
	"time"
)

func waitAck(ctx context.Context, conn syscall.Conn, fallbackDelay time.Duration) error {
	time.Sleep(fallbackDelay)
	return nil
}
