//go:build linux

package usbip

import (
	"bytes"

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
		Groups: 1,
	}
	err = unix.Bind(fd, addr)
	if err != nil {
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

var usbSubsystemMarker = []byte("\x00SUBSYSTEM=usb\x00")

func isUSBUEvent(raw []byte) bool {
	return bytes.Contains(raw, usbSubsystemMarker)
}
