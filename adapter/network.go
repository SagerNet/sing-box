package adapter

import (
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/control"
)

type NetworkManager interface {
	NewService
	InterfaceFinder() control.InterfaceFinder
	UpdateInterfaces() error
	DefaultInterface() string
	AutoDetectInterface() bool
	AutoDetectInterfaceFunc() control.Func
	DefaultMark() uint32
	RegisterAutoRedirectOutputMark(mark uint32) error
	AutoRedirectOutputMark() uint32
	NetworkMonitor() tun.NetworkUpdateMonitor
	InterfaceMonitor() tun.DefaultInterfaceMonitor
	PackageManager() tun.PackageManager
	WIFIState() WIFIState
	ResetNetwork()
}
