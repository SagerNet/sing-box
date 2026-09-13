//go:build !(darwin || dragonfly || freebsd || netbsd || openbsd)

package listener

import (
	C "github.com/sagernet/sing-box/constant"
)

func UDPSocketBufferSize() int {
	return C.UDPSocketBufferSize
}
