package wireguard

import (
	"context"
	"net/netip"
	"time"

	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/control"
	"github.com/sagernet/sing/common/logger"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/wireguard-go/device"
	wgTun "github.com/sagernet/wireguard-go/tun"
)

type Device interface {
	wgTun.Device
	N.Dialer
	Start() error
	SetDevice(device *device.Device, peers []*device.Peer)
	Inet4Address() netip.Addr
	Inet6Address() netip.Addr
}

type DeviceOptions struct {
	Context         context.Context
	Logger          logger.ContextLogger
	System          bool
	Handler         tun.Handler
	UDPTimeout      time.Duration
	ICMPTimeout     time.Duration
	UDPMapping      tun.NATMapping
	UDPFiltering    tun.NATFiltering
	UDPNATMax       uint32
	InterfaceFinder control.InterfaceFinder
	MemoryPressure  func() tun.MemoryPressure
	CreateDialer    func(interfaceName string) N.Dialer
	Name            string
	MTU             uint32
	Address         []netip.Prefix
	AllowedAddress  []netip.Prefix
}

func NewDevice(options DeviceOptions) (Device, error) {
	if !options.System {
		return newStackDevice(options)
	}
	return newSystemStackDevice(options)
}

func newStack(options DeviceOptions, memoryTun *tun.MemoryTun) (*tun.Go, error) {
	return tun.NewGo(tun.StackOptions{
		Context:         options.Context,
		Tun:             memoryTun,
		TunOptions:      tun.Options{MTU: options.MTU},
		UDPTimeout:      options.UDPTimeout,
		ICMPTimeout:     options.ICMPTimeout,
		UDPMapping:      options.UDPMapping,
		UDPFiltering:    options.UDPFiltering,
		UDPNATMax:       options.UDPNATMax,
		Handler:         options.Handler,
		Logger:          options.Logger,
		InterfaceFinder: options.InterfaceFinder,
		MemoryPressure:  options.MemoryPressure,
	})
}

func deviceAddresses(addresses []netip.Prefix) (netip.Addr, netip.Addr) {
	inet4Prefix := common.Find(addresses, func(it netip.Prefix) bool { return it.Addr().Is4() })
	inet6Prefix := common.Find(addresses, func(it netip.Prefix) bool { return it.Addr().Is6() })
	return inet4Prefix.Addr(), inet6Prefix.Addr()
}
