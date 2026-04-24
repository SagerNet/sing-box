//go:build linux

package usbip

import (
	"bytes"

	"golang.org/x/sys/unix"
)

const ueventReceiveBufferSize = 1 << 20

type ueventListener struct {
	fd int
}

func newUEventListener() (*ueventListener, error) {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_DGRAM, unix.NETLINK_KOBJECT_UEVENT)
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
	return &ueventListener{fd: fd}, nil
}

func (l *ueventListener) Close() error {
	return unix.Close(l.fd)
}

func (l *ueventListener) WaitUSBEvent() error {
	var buf [16384]byte
	for {
		n, from, err := unix.Recvfrom(l.fd, buf[:], 0)
		if err == unix.ENOBUFS {
			return nil
		}
		if err != nil {
			return err
		}
		if source, ok := from.(*unix.SockaddrNetlink); ok && source.Pid != 0 {
			continue
		}
		if isUSBDeviceUEvent(buf[:n]) {
			return nil
		}
	}
}

var usbSubsystemMarker = []byte("\x00SUBSYSTEM=usb\x00")
var usbDeviceTypeMarker = []byte("\x00DEVTYPE=usb_device\x00")

func isUSBDeviceUEvent(raw []byte) bool {
	return bytes.Contains(raw, usbSubsystemMarker) && bytes.Contains(raw, usbDeviceTypeMarker)
}
