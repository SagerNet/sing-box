//go:build linux

package process

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net/netip"
	"os"
	"path"
	"strings"
	"syscall"
	"unicode"
	"unsafe"

	"github.com/sagernet/netlink"
	N "github.com/sagernet/sing/common/network"
)

// from https://github.com/vishvananda/netlink/blob/bca67dfc8220b44ef582c9da4e9172bf1c9ec973/nl/nl_linux.go#L52-L62
var nativeEndian = func() binary.ByteOrder {
	var x uint32 = 0x01020304
	if *(*byte)(unsafe.Pointer(&x)) == 0x01 {
		return binary.BigEndian
	}

	return binary.LittleEndian
}()

const (
	sizeOfSocketDiagRequest = syscall.SizeofNlMsghdr + 8 + 48
	socketDiagByFamily      = 20
	pathProc                = "/proc"
)

func resolveSocketByNetlink(network string, source netip.AddrPort, destination netip.AddrPort) (*netlink.Socket, error) {
	var family uint8
	var protocol uint8

	switch network {
	case N.NetworkTCP:
		protocol = syscall.IPPROTO_TCP
	case N.NetworkUDP:
		protocol = syscall.IPPROTO_UDP
	default:
		return nil, os.ErrInvalid
	}
	if source.Addr().Is4() {
		family = syscall.AF_INET
	} else {
		family = syscall.AF_INET6
	}
	sockets, err := netlink.SocketGet(family, protocol, source, netip.AddrPortFrom(netip.IPv6Unspecified(), 0))
	if err == nil {
		sockets, err = netlink.SocketGet(family, protocol, source, destination)
	}
	if err != nil {
		return nil, err
	}
	if len(sockets) > 1 {
		for _, socket := range sockets {
			if socket.ID.DestinationPort == destination.Port() {
				return socket, nil
			}
		}
	}
	return sockets[0], nil
}

func resolveProcessNameByProcSearch(inode, uid uint32) (string, error) {
	files, err := os.ReadDir(pathProc)
	if err != nil {
		return "", err
	}

	buffer := make([]byte, syscall.PathMax)
	socket := []byte(fmt.Sprintf("socket:[%d]", inode))

	for _, f := range files {
		if !f.IsDir() || !isPid(f.Name()) {
			continue
		}

		info, err := f.Info()
		if err != nil {
			return "", err
		}
		if info.Sys().(*syscall.Stat_t).Uid != uid {
			continue
		}

		processPath := path.Join(pathProc, f.Name())
		fdPath := path.Join(processPath, "fd")

		fds, err := os.ReadDir(fdPath)
		if err != nil {
			continue
		}

		for _, fd := range fds {
			n, err := syscall.Readlink(path.Join(fdPath, fd.Name()), buffer)
			if err != nil {
				continue
			}

			if bytes.Equal(buffer[:n], socket) {
				return os.Readlink(path.Join(processPath, "exe"))
			}
		}
	}

	return "", fmt.Errorf("process of uid(%d),inode(%d) not found", uid, inode)
}

func isPid(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool {
		return !unicode.IsDigit(r)
	}) == -1
}
