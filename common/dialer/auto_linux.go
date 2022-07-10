package dialer

import (
	"syscall"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
)

func BindToInterface(router adapter.Router) control.Func {
	return func(network, address string, conn syscall.RawConn) error {
		interfaceName := router.DefaultInterfaceName()
		if interfaceName == "" {
			return nil
		}
		var innerErr error
		err := conn.Control(func(fd uintptr) {
			innerErr = syscall.BindToDevice(int(fd), interfaceName)
		})
		return E.Errors(innerErr, err)
	}
}
