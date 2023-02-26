package libbox

import (
	"golang.org/x/sys/unix"
)

// kanged from wireauard-apple

const utunControlName = "com.apple.net.utun_control"

func GetTunnelFileDescriptor() int32 {
	ctlInfo := &unix.CtlInfo{}
	copy(ctlInfo.Name[:], utunControlName)
	for fd := 0; fd < 1024; fd++ {
		addr, err := unix.Getpeername(fd)
		if err != nil {
			continue
		}
		addrCTL, loaded := addr.(*unix.SockaddrCtl)
		if !loaded {
			continue
		}
		if ctlInfo.Id == 0 {
			err = unix.IoctlCtlInfo(fd, ctlInfo)
			if err != nil {
				continue
			}
		}
		if addrCTL.ID == ctlInfo.Id {
			return int32(fd)
		}
	}
	return -1
}
