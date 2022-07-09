//go:build linux

package tun

import (
	"gvisor.dev/gvisor/pkg/tcpip/link/fdbased"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

func NewEndpoint(tunFd uintptr, tunMtu uint32) (stack.LinkEndpoint, error) {
	return fdbased.New(&fdbased.Options{
		FDs:                []int{int(tunFd)},
		MTU:                tunMtu,
		PacketDispatchMode: fdbased.PacketMMap,
	})
}
