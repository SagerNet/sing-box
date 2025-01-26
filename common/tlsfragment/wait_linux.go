package tf

import (
	"context"
	"syscall"
	"time"

	"github.com/sagernet/sing/common/control"

	"golang.org/x/sys/unix"
)

func waitAck(ctx context.Context, conn syscall.Conn, fallbackDelay time.Duration) error {
	return control.Conn(conn, func(fd uintptr) error {
		start := time.Now()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			tcpInfo, err := unix.GetsockoptTCPInfo(int(fd), unix.IPPROTO_TCP, unix.TCP_INFO)
			if err != nil {
				return err
			}
			if tcpInfo.Unacked == 0 {
				if time.Since(start) <= 20*time.Millisecond {
					// under transparent proxy
					time.Sleep(fallbackDelay)
				}
				return nil
			}
			time.Sleep(10 * time.Millisecond)
		}
	})
}
