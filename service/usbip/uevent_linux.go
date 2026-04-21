//go:build linux

package usbip

import (
	"bytes"
	"os"

	"golang.org/x/sys/unix"
)

type ueventListener struct {
	fd int
}

func newUEventListener() (*ueventListener, error) {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_DGRAM, unix.NETLINK_KOBJECT_UEVENT)
	if err != nil {
		return nil, err
	}
	addr := &unix.SockaddrNetlink{
		Family: unix.AF_NETLINK,
		Pid:    uint32(os.Getpid()),
		Groups: 1,
	}
	if err := unix.Bind(fd, addr); err != nil {
		_ = unix.Close(fd)
		return nil, err
	}
	return &ueventListener{fd: fd}, nil
}

func (l *ueventListener) Close() error {
	return unix.Close(l.fd)
}

func (l *ueventListener) WaitUSBEvent() error {
	var buf [4096]byte
	for {
		n, _, err := unix.Recvfrom(l.fd, buf[:], 0)
		if err != nil {
			return err
		}
		if isUSBUEvent(buf[:n]) {
			return nil
		}
	}
}

func isUSBUEvent(raw []byte) bool {
	fields := bytes.Split(bytes.TrimRight(raw, "\x00"), []byte{0})
	for _, field := range fields {
		if bytes.Equal(field, []byte("SUBSYSTEM=usb")) {
			return true
		}
	}
	return false
}
