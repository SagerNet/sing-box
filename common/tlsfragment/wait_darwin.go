package tf

import (
	"context"
	"net"
	"time"

	"github.com/sagernet/sing/common/control"

	"golang.org/x/sys/unix"
)

/*
const tcpMaxNotifyAck = 10

type tcpNotifyAckID uint32

	type tcpNotifyAckComplete struct {
		NotifyPending       uint32
		NotifyCompleteCount uint32
		NotifyCompleteID    [tcpMaxNotifyAck]tcpNotifyAckID
	}

var sizeOfTCPNotifyAckComplete = int(unsafe.Sizeof(tcpNotifyAckComplete{}))

	func getsockoptTCPNotifyAckComplete(fd, level, opt int) (*tcpNotifyAckComplete, error) {
		var value tcpNotifyAckComplete
		vallen := uint32(sizeOfTCPNotifyAckComplete)
		err := getsockopt(fd, level, opt, unsafe.Pointer(&value), &vallen)
		return &value, err
	}

//go:linkname getsockopt golang.org/x/sys/unix.getsockopt
func getsockopt(s int, level int, name int, val unsafe.Pointer, vallen *uint32) error

	func waitAck(ctx context.Context, conn *net.TCPConn, _ time.Duration) error {
		const TCP_NOTIFY_ACKNOWLEDGEMENT = 0x212
		return control.Conn(conn, func(fd uintptr) error {
			err := unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, TCP_NOTIFY_ACKNOWLEDGEMENT, 1)
			if err != nil {
				if errors.Is(err, unix.EINVAL) {
					return waitAckFallback(ctx, conn, 0)
				}
				return err
			}
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
				var ackComplete *tcpNotifyAckComplete
				ackComplete, err = getsockoptTCPNotifyAckComplete(int(fd), unix.IPPROTO_TCP, TCP_NOTIFY_ACKNOWLEDGEMENT)
				if err != nil {
					return err
				}
				if ackComplete.NotifyPending == 0 {
					return nil
				}
				time.Sleep(10 * time.Millisecond)
			}
		})
	}
*/

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
			unacked, err := unix.GetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_NWRITE)
			if err != nil {
				return err
			}
			if unacked == 0 {
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
