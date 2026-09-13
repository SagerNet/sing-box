//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package listener

import (
	"sync"

	C "github.com/sagernet/sing-box/constant"

	"golang.org/x/sys/unix"
)

var UDPSocketBufferSize = sync.OnceValue(func() int {
	maxSocketBuffer, err := unix.SysctlUint32("kern.ipc.maxsockbuf")
	if err != nil {
		return C.UDPSocketBufferSize
	}
	// xnu sbreserve() before xnu-11215 rejects cc > sb_max * MCLBYTES / (MSIZE + MCLBYTES) with ENOBUFS instead of clamping.
	return int(uint64(maxSocketBuffer) * 2048 / (256 + 2048))
})
