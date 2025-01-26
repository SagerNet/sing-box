package tf

import (
	"context"
	"net"
	"time"

	"github.com/sagernet/sing/common/control"

	"golang.org/x/sys/unix"
)

func writeAndWaitAck(ctx context.Context, conn *net.TCPConn, payload []byte, fallbackDelay time.Duration) error {
	_, err := conn.Write(payload)
	if err != nil {
		return err
	}
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
