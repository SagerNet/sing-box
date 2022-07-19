//go:build (linux || windows) && !no_gvisor

package route

import (
	"github.com/sagernet/sing-tun"
	E "github.com/sagernet/sing/common/exceptions"
)

func NewNetworkUpdateMonitor(errorHandler E.Handler) (NetworkUpdateMonitor, error) {
	return tun.NewNetworkUpdateMonitor(errorHandler)
}

func NewDefaultInterfaceMonitor(networkMonitor NetworkUpdateMonitor, callback DefaultInterfaceUpdateCallback) (DefaultInterfaceMonitor, error) {
	return tun.NewDefaultInterfaceMonitor(networkMonitor, callback)
}
