//go:build linux

package usbip

import (
	"bytes"
	"os"

	"golang.org/x/sys/unix"
)

const ueventReceiveBufferSize = 1 << 20

// ueventListener wraps the netlink socket in an *os.File so the fd is
// owned by the Go runtime poller: Close wakes a goroutine blocked in
// WaitUSBEvent, is idempotent, and the fd number cannot be recycled to
// another connection while a read is still in flight — all of which a
// raw fd with blocking Recvfrom gets wrong.
type ueventListener struct {
	file *os.File
}

func newUEventListener() (*ueventListener, error) {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_DGRAM|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, unix.NETLINK_KOBJECT_UEVENT)
	if err != nil {
		return nil, err
	}
	_ = unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_RCVBUF, ueventReceiveBufferSize)
	addr := &unix.SockaddrNetlink{
		Family: unix.AF_NETLINK,
		Groups: 1,
	}
	err = unix.Bind(fd, addr)
	if err != nil {
		_ = unix.Close(fd)
		return nil, err
	}
	return &ueventListener{file: os.NewFile(uintptr(fd), "netlink-uevent")}, nil
}

func (l *ueventListener) Close() error {
	return l.file.Close()
}

func (l *ueventListener) WaitUSBEvent() error {
	rawConn, err := l.file.SyscallConn()
	if err != nil {
		return err
	}
	var buf [16384]byte
	for {
		var (
			n       int
			from    unix.Sockaddr
			recvErr error
		)
		err = rawConn.Read(func(fd uintptr) bool {
			for {
				n, from, recvErr = unix.Recvfrom(int(fd), buf[:], 0)
				if recvErr == unix.EINTR {
					continue
				}
				return recvErr != unix.EAGAIN
			}
		})
		if err != nil {
			return err
		}
		if recvErr == unix.ENOBUFS {
			return nil
		}
		if recvErr != nil {
			return recvErr
		}
		if source, ok := from.(*unix.SockaddrNetlink); ok && source.Pid != 0 {
			continue
		}
		if isUSBDeviceUEvent(buf[:n]) {
			return nil
		}
	}
}

var (
	usbSubsystemMarker  = []byte("\x00SUBSYSTEM=usb\x00")
	usbDeviceTypeMarker = []byte("\x00DEVTYPE=usb_device\x00")
)

func isUSBDeviceUEvent(raw []byte) bool {
	return bytes.Contains(raw, usbSubsystemMarker) && bytes.Contains(raw, usbDeviceTypeMarker)
}
