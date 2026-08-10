//go:build linux

package process

import (
	"encoding/binary"
	"errors"
	"net/netip"
	"os"
	"sync"
	"syscall"

	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
)

const (
	sizeOfSocketDiagRequestData = 56
	sizeOfSocketDiagRequest     = syscall.SizeofNlMsghdr + sizeOfSocketDiagRequestData
	socketDiagResponseMinSize   = 72
	socketDiagByFamily          = 20
)

type socketDiagConn struct {
	access   sync.Mutex
	family   uint8
	protocol uint8
	fd       int
}

func socketDiagConnIndex(family, protocol uint8) int {
	index := 0
	if protocol == syscall.IPPROTO_UDP {
		index += 2
	}
	if family == syscall.AF_INET6 {
		index++
	}
	return index
}

func socketDiagSettings(network string, source netip.AddrPort) (family, protocol uint8, err error) {
	switch network {
	case N.NetworkTCP:
		protocol = syscall.IPPROTO_TCP
	case N.NetworkUDP:
		protocol = syscall.IPPROTO_UDP
	default:
		return 0, 0, os.ErrInvalid
	}
	switch {
	case source.Addr().Is4():
		family = syscall.AF_INET
	case source.Addr().Is6():
		family = syscall.AF_INET6
	default:
		return 0, 0, os.ErrInvalid
	}
	return family, protocol, nil
}

func (c *socketDiagConn) Close() error {
	c.access.Lock()
	defer c.access.Unlock()
	return c.closeLocked()
}

func (c *socketDiagConn) query(source netip.AddrPort, destination netip.AddrPort) (inode, uid uint32, err error) {
	c.access.Lock()
	defer c.access.Unlock()
	request := packSocketDiagRequest(c.family, c.protocol, source, destination, false)
	for range 2 {
		err = c.ensureOpenLocked()
		if err != nil {
			return 0, 0, E.Cause(err, "dial netlink")
		}
		inode, uid, err = querySocketDiag(c.fd, request)
		if err == nil || errors.Is(err, ErrNotFound) {
			return inode, uid, err
		}
		_ = c.closeLocked()
	}
	return 0, 0, err
}

func querySocketDiagOnce(family, protocol uint8, source netip.AddrPort) (inode, uid uint32, err error) {
	fd, err := openSocketDiag()
	if err != nil {
		return 0, 0, E.Cause(err, "dial netlink")
	}
	defer syscall.Close(fd)
	return querySocketDiag(fd, packSocketDiagRequest(family, protocol, source, netip.AddrPort{}, true))
}

func (c *socketDiagConn) ensureOpenLocked() error {
	if c.fd != -1 {
		return nil
	}
	fd, err := openSocketDiag()
	if err != nil {
		return err
	}
	c.fd = fd
	return nil
}

func openSocketDiag() (int, error) {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_DGRAM|syscall.SOCK_CLOEXEC, syscall.NETLINK_INET_DIAG)
	if err != nil {
		return -1, err
	}
	timeout := &syscall.Timeval{Usec: 100}
	if err = syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_SNDTIMEO, timeout); err != nil {
		syscall.Close(fd)
		return -1, err
	}
	if err = syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, timeout); err != nil {
		syscall.Close(fd)
		return -1, err
	}
	if err = syscall.Connect(fd, &syscall.SockaddrNetlink{
		Family: syscall.AF_NETLINK,
		Pid:    0,
		Groups: 0,
	}); err != nil {
		syscall.Close(fd)
		return -1, err
	}
	return fd, nil
}

func (c *socketDiagConn) closeLocked() error {
	if c.fd == -1 {
		return nil
	}
	err := syscall.Close(c.fd)
	c.fd = -1
	return err
}

func packSocketDiagRequest(family, protocol byte, source netip.AddrPort, destination netip.AddrPort, dump bool) []byte {
	request := make([]byte, sizeOfSocketDiagRequest)

	binary.NativeEndian.PutUint32(request[0:4], sizeOfSocketDiagRequest)
	binary.NativeEndian.PutUint16(request[4:6], socketDiagByFamily)
	flags := uint16(syscall.NLM_F_REQUEST)
	if dump {
		flags |= syscall.NLM_F_DUMP
	}
	binary.NativeEndian.PutUint16(request[6:8], flags)
	binary.NativeEndian.PutUint32(request[8:12], 0)
	binary.NativeEndian.PutUint32(request[12:16], 0)

	request[16] = family
	request[17] = protocol
	request[18] = 0
	request[19] = 0
	if dump {
		binary.NativeEndian.PutUint32(request[20:24], 0xFFFFFFFF)
	}
	requestSource := source
	requestDestination := destination
	if protocol == syscall.IPPROTO_UDP && !dump && destination.IsValid() {
		// udp_dump_one expects the exact-match endpoints reversed for historical reasons.
		requestSource, requestDestination = destination, source
	}
	binary.BigEndian.PutUint16(request[24:26], requestSource.Port())
	binary.BigEndian.PutUint16(request[26:28], requestDestination.Port())
	if family == syscall.AF_INET6 {
		copy(request[28:44], requestSource.Addr().AsSlice())
		if requestDestination.IsValid() {
			copy(request[44:60], requestDestination.Addr().AsSlice())
		}
	} else {
		copy(request[28:32], requestSource.Addr().AsSlice())
		if requestDestination.IsValid() {
			copy(request[44:48], requestDestination.Addr().AsSlice())
		}
	}
	binary.NativeEndian.PutUint32(request[60:64], 0)
	binary.NativeEndian.PutUint64(request[64:72], 0xFFFFFFFFFFFFFFFF)
	return request
}

func querySocketDiag(fd int, request []byte) (inode, uid uint32, err error) {
	_, err = syscall.Write(fd, request)
	if err != nil {
		return 0, 0, E.Cause(err, "write netlink request")
	}
	buffer := make([]byte, 64<<10)
	n, err := syscall.Read(fd, buffer)
	if err != nil {
		return 0, 0, E.Cause(err, "read netlink response")
	}
	messages, err := syscall.ParseNetlinkMessage(buffer[:n])
	if err != nil {
		return 0, 0, E.Cause(err, "parse netlink message")
	}
	return unpackSocketDiagMessages(messages)
}

func unpackSocketDiagMessages(messages []syscall.NetlinkMessage) (inode, uid uint32, err error) {
	for _, message := range messages {
		switch message.Header.Type {
		case syscall.NLMSG_DONE:
			continue
		case syscall.NLMSG_ERROR:
			err = unpackSocketDiagError(&message)
			if err != nil {
				return 0, 0, err
			}
		case socketDiagByFamily:
			inode, uid = unpackSocketDiagResponse(&message)
			if inode != 0 || uid != 0 {
				return inode, uid, nil
			}
		}
	}
	return 0, 0, ErrNotFound
}

func unpackSocketDiagResponse(msg *syscall.NetlinkMessage) (inode, uid uint32) {
	if len(msg.Data) < socketDiagResponseMinSize {
		return 0, 0
	}
	uid = binary.NativeEndian.Uint32(msg.Data[64:68])
	inode = binary.NativeEndian.Uint32(msg.Data[68:72])
	return inode, uid
}

func unpackSocketDiagError(msg *syscall.NetlinkMessage) error {
	if len(msg.Data) < 4 {
		return E.New("netlink message: NLMSG_ERROR")
	}
	errno := int32(binary.NativeEndian.Uint32(msg.Data[:4]))
	if errno == 0 {
		return nil
	}
	if errno < 0 {
		errno = -errno
	}
	sysErr := syscall.Errno(errno)
	switch sysErr {
	case syscall.ENOENT, syscall.ESRCH:
		return ErrNotFound
	default:
		return E.New("netlink message: ", sysErr)
	}
}
