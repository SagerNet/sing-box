//go:build !(linux || windows) || no_gvisor

package route

import (
	"os"

	E "github.com/sagernet/sing/common/exceptions"
)

func NewNetworkUpdateMonitor(errorHandler E.Handler) (NetworkUpdateMonitor, error) {
	return nil, os.ErrInvalid
}

func NewDefaultInterfaceMonitor(networkMonitor NetworkUpdateMonitor, callback DefaultInterfaceUpdateCallback) (DefaultInterfaceMonitor, error) {
	return nil, os.ErrInvalid
}
