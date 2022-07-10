package iffmonitor

import "github.com/sagernet/sing-box/adapter"

type InterfaceMonitor interface {
	adapter.Service
	DefaultInterfaceName() string
	DefaultInterfaceIndex() int
}
