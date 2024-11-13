package adapter

import (
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/control"
)

type NetworkManager interface {
	Lifecycle
	InterfaceFinder() control.InterfaceFinder
	UpdateInterfaces() error
	DefaultNetworkInterface() *NetworkInterface
	NetworkInterfaces() []NetworkInterface
	AutoDetectInterface() bool
	AutoDetectInterfaceFunc() control.Func
	ProtectFunc() control.Func
	DefaultOptions() NetworkOptions
	RegisterAutoRedirectOutputMark(mark uint32) error
	AutoRedirectOutputMark() uint32
	NetworkMonitor() tun.NetworkUpdateMonitor
	InterfaceMonitor() tun.DefaultInterfaceMonitor
	PackageManager() tun.PackageManager
	WIFIState() WIFIState
	ResetNetwork()
}

type NetworkOptions struct {
	NetworkStrategy     C.NetworkStrategy
	NetworkType         []C.InterfaceType
	FallbackNetworkType []C.InterfaceType
	FallbackDelay       time.Duration
	BindInterface       string
	RoutingMark         uint32
}

type InterfaceUpdateListener interface {
	InterfaceUpdated()
}

type WIFIState struct {
	SSID  string
	BSSID string
}

type NetworkInterface struct {
	control.Interface
	Type        C.InterfaceType
	DNSServers  []string
	Expensive   bool
	Constrained bool
}
