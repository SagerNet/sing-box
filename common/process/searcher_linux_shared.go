//go:build linux

package process

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path"
	"strings"
	"syscall"
	"unicode"
	"unsafe"

	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
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

func resolveSocketByNetlink(network string, source netip.AddrPort, destination netip.AddrPort) (inode, uid uint32, err error) {
	var family uint8
	var protocol uint8

	switch network {
	case N.NetworkTCP:
		protocol = syscall.IPPROTO_TCP
	case N.NetworkUDP:
		protocol = syscall.IPPROTO_UDP
	default:
		return 0, 0, os.ErrInvalid
	}

	if source.Addr().Is4() {
		family = syscall.AF_INET
	} else {
		family = syscall.AF_INET6
	}

	req := packSocketDiagRequest(family, protocol, source)

	socket, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_DGRAM, syscall.NETLINK_INET_DIAG)
	if err != nil {
		return 0, 0, E.Cause(err, "dial netlink")
	}
	defer syscall.Close(socket)

	syscall.SetsockoptTimeval(socket, syscall.SOL_SOCKET, syscall.SO_SNDTIMEO, &syscall.Timeval{Usec: 100})
	syscall.SetsockoptTimeval(socket, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &syscall.Timeval{Usec: 100})

	err = syscall.Connect(socket, &syscall.SockaddrNetlink{
		Family: syscall.AF_NETLINK,
		Pad:    0,
		Pid:    0,
		Groups: 0,
	})
	if err != nil {
		return
	}

	_, err = syscall.Write(socket, req)
	if err != nil {
		return 0, 0, E.Cause(err, "write netlink request")
	}

	buffer := buf.New()
	defer buffer.Release()

	n, err := syscall.Read(socket, buffer.FreeBytes())
	if err != nil {
		return 0, 0, E.Cause(err, "read netlink response")
	}

	buffer.Truncate(n)

	messages, err := syscall.ParseNetlinkMessage(buffer.Bytes())
	if err != nil {
		return 0, 0, E.Cause(err, "parse netlink message")
	} else if len(messages) == 0 {
		return 0, 0, E.New("unexcepted netlink response")
	}

	message := messages[0]
	if message.Header.Type&syscall.NLMSG_ERROR != 0 {
		return 0, 0, E.New("netlink message: NLMSG_ERROR")
	}

	inode, uid = unpackSocketDiagResponse(&messages[0])
	return
}

func packSocketDiagRequest(family, protocol byte, source netip.AddrPort) []byte {
	s := make([]byte, 16)
	copy(s, source.Addr().AsSlice())

	buf := make([]byte, sizeOfSocketDiagRequest)

	nativeEndian.PutUint32(buf[0:4], sizeOfSocketDiagRequest)
	nativeEndian.PutUint16(buf[4:6], socketDiagByFamily)
	nativeEndian.PutUint16(buf[6:8], syscall.NLM_F_REQUEST|syscall.NLM_F_DUMP)
	nativeEndian.PutUint32(buf[8:12], 0)
	nativeEndian.PutUint32(buf[12:16], 0)

	buf[16] = family
	buf[17] = protocol
	buf[18] = 0
	buf[19] = 0
	nativeEndian.PutUint32(buf[20:24], 0xFFFFFFFF)

	binary.BigEndian.PutUint16(buf[24:26], source.Port())
	binary.BigEndian.PutUint16(buf[26:28], 0)

	copy(buf[28:44], s)
	copy(buf[44:60], net.IPv6zero)

	nativeEndian.PutUint32(buf[60:64], 0)
	nativeEndian.PutUint64(buf[64:72], 0xFFFFFFFFFFFFFFFF)

	return buf
}

func unpackSocketDiagResponse(msg *syscall.NetlinkMessage) (inode, uid uint32) {
	if len(msg.Data) < 72 {
		return 0, 0
	}

	data := msg.Data

	uid = nativeEndian.Uint32(data[64:68])
	inode = nativeEndian.Uint32(data[68:72])

	return
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
