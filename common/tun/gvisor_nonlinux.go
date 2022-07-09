//go:build !linux

package tun

import "gvisor.dev/gvisor/pkg/tcpip/stack"

func NewEndpoint(tunFd uintptr, tunMtu uint32) (stack.LinkEndpoint, error) {
	return NewPosixEndpoint(tunFd, tunMtu)
}
