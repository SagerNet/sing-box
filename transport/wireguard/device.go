package wireguard

import (
	"context"
	"net/netip"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/logger"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/wireguard-go/device"
	wgTun "github.com/sagernet/wireguard-go/tun"
)

type Device interface {
	wgTun.Device
	N.Dialer
	Start() error
	SetDevice(device *device.Device)
	Inet4Address() netip.Addr
	Inet6Address() netip.Addr
}

type DeviceOptions struct {
	Context        context.Context
	Logger         logger.ContextLogger
	System         bool
	Handler        tun.Handler
	UDPTimeout     time.Duration
	CreateDialer   func(interfaceName string) N.Dialer
	Name           string
	MTU            uint32
	Address        []netip.Prefix
	AllowedAddress []netip.Prefix
}

func NewDevice(options DeviceOptions) (Device, error) {
	if !options.System {
		return newStackDevice(options)
	} else if !tun.WithGVisor {
		return newSystemDevice(options)
	} else {
		return newSystemStackDevice(options)
	}
}

type NatDevice interface {
	Device
	CreateDestination(metadata adapter.InboundContext, routeContext tun.DirectRouteContext, timeout time.Duration) (tun.DirectRouteDestination, error)
}
