package dialer

import (
	"syscall"

	"github.com/sagernet/sing/common/control"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func skipIfPrivate(next control.Func) control.Func {
	return func(network, address string, conn syscall.RawConn) error {
		destination := M.ParseSocksaddr(address)
		if !N.IsPublicAddr(destination.Addr) {
			return nil
		}
		return next(network, address, conn)
	}
}
