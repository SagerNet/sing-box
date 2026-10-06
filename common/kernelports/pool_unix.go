//go:build !windows

package kernelports

import (
	"errors"
	"net/netip"
	"sync/atomic"
	"syscall"

	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing-tun/gtcpip/header"

	"golang.org/x/sys/unix"
)

const lowestSelector = 1024

type Pool struct {
	tcp atomic.Pointer[[]tun.SelectorRange]
	udp atomic.Pointer[[]tun.SelectorRange]
}

func New() (*Pool, error) {
	first, last := ephemeralRange()
	below := tun.SelectorRange{Start: lowestSelector}
	if first > lowestSelector {
		below.Count = first - lowestSelector
	}
	above := tun.SelectorRange{Start: last + 1, Count: 65535 - last}
	selected := below
	if above.Count > below.Count {
		selected = above
	}
	if selected.Count == 0 {
		selected = tun.SelectorRange{Start: 49152, Count: 65535 - 49152 + 1}
	}
	ranges := []tun.SelectorRange{selected}
	pool := &Pool{}
	pool.tcp.Store(&ranges)
	pool.udp.Store(&ranges)
	return pool, nil
}

func (p *Pool) Acquire(protocol uint8) (tun.SelectorRange, error) {
	return tun.SelectorRange{}, ErrExhausted
}

func (p *Pool) ReserveSelector(protocol uint8, address netip.AddrPort) bool {
	var socketType int
	switch protocol {
	case uint8(header.TCPProtocolNumber):
		socketType = unix.SOCK_STREAM
	case uint8(header.UDPProtocolNumber):
		socketType = unix.SOCK_DGRAM
	default:
		return true
	}
	family := unix.AF_INET
	var sockaddr unix.Sockaddr
	if address.Addr().Is4() {
		sockaddr4 := &unix.SockaddrInet4{Port: int(address.Port())}
		sockaddr4.Addr = address.Addr().As4()
		sockaddr = sockaddr4
	} else {
		family = unix.AF_INET6
		sockaddr6 := &unix.SockaddrInet6{Port: int(address.Port())}
		sockaddr6.Addr = address.Addr().As16()
		sockaddr = sockaddr6
	}
	syscall.ForkLock.RLock()
	fd, err := unix.Socket(family, socketType, 0)
	if err == nil {
		unix.CloseOnExec(fd)
	}
	syscall.ForkLock.RUnlock()
	if err != nil {
		return true
	}
	defer unix.Close(fd)
	err = unix.Bind(fd, sockaddr)
	return !errors.Is(err, unix.EADDRINUSE)
}

func (p *Pool) Close() error {
	return nil
}
