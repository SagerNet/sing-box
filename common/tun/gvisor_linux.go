//go:build linux

package tun

import (
	"runtime"

	"gvisor.dev/gvisor/pkg/tcpip/link/fdbased"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

func NewEndpoint(tunFd uintptr, tunMtu uint32) (stack.LinkEndpoint, error) {
	var packetDispatchMode fdbased.PacketDispatchMode
	if runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64" {
		packetDispatchMode = fdbased.PacketMMap
	} else {
		packetDispatchMode = fdbased.RecvMMsg
	}
	return fdbased.New(&fdbased.Options{
		FDs:                []int{int(tunFd)},
		MTU:                tunMtu,
		PacketDispatchMode: packetDispatchMode,
	})
}
