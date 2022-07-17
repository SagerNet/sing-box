package route

import "github.com/sagernet/sing/common/x/list"

type (
	NetworkUpdateCallback          = func() error
	DefaultInterfaceUpdateCallback = func()
)

type NetworkUpdateMonitor interface {
	Start() error
	Close() error
	RegisterCallback(callback NetworkUpdateCallback) *list.Element[NetworkUpdateCallback]
	UnregisterCallback(element *list.Element[NetworkUpdateCallback])
}

type DefaultInterfaceMonitor interface {
	Start() error
	Close() error
	DefaultInterfaceName() string
	DefaultInterfaceIndex() int
}
