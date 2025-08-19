//go:build !(linux || darwin || windows)

package tf

import (
	"context"
	"net"
	"time"
)

func writeAndWaitAck(ctx context.Context, conn *net.TCPConn, payload []byte, fallbackDelay time.Duration) error {
	_, err := conn.Write(payload)
	if err != nil {
		return err
	}
	time.Sleep(fallbackDelay)
	return nil
}
