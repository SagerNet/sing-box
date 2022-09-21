package dialer

import (
	"context"
	"net"
	"net/netip"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/warning"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/database64128/tfo-go"
)

var warnBindInterfaceOnUnsupportedPlatform = warning.New(
	func() bool {
		return !(C.IsLinux || C.IsWindows || C.IsDarwin)
	},
	"outbound option `bind_interface` is only supported on Linux and Windows",
)

var warnRoutingMarkOnUnsupportedPlatform = warning.New(
	func() bool {
		return !C.IsLinux
	},
	"outbound option `routing_mark` is only supported on Linux",
)

var warnReuseAdderOnUnsupportedPlatform = warning.New(
	func() bool {
		return !(C.IsDarwin || C.IsDragonfly || C.IsFreebsd || C.IsLinux || C.IsNetbsd || C.IsOpenbsd || C.IsSolaris || C.IsWindows)
	},
	"outbound option `reuse_addr` is unsupported on current platform",
)

var warnProtectPathOnNonAndroid = warning.New(
	func() bool {
		return !C.IsAndroid
	},
	"outbound option `protect_path` is only supported on Android",
)

var warnTFOOnUnsupportedPlatform = warning.New(
	func() bool {
		return !(C.IsDarwin || C.IsFreebsd || C.IsLinux || C.IsWindows)
	},
	"outbound option `tcp_fast_open` is unsupported on current platform",
)

type DefaultDialer struct {
	dialer      tfo.Dialer
	udpDialer   net.Dialer
	udpListener net.ListenConfig
	bindUDPAddr string
}

func NewDefault(router adapter.Router, options option.DialerOptions) *DefaultDialer {
	var dialer net.Dialer
	var listener net.ListenConfig
	if options.BindInterface != "" {
		warnBindInterfaceOnUnsupportedPlatform.Check()
		bindFunc := control.BindToInterface(router.InterfaceFinder(), options.BindInterface, -1)
		dialer.Control = control.Append(dialer.Control, bindFunc)
		listener.Control = control.Append(listener.Control, bindFunc)
	} else if router.AutoDetectInterface() {
		const useInterfaceName = C.IsLinux
		bindFunc := control.BindToInterfaceFunc(router.InterfaceFinder(), func(network string, address string) (interfaceName string, interfaceIndex int) {
			remoteAddr := M.ParseSocksaddr(address).Addr
			if C.IsLinux {
				return router.InterfaceMonitor().DefaultInterfaceName(remoteAddr), -1
			} else {
				return "", router.InterfaceMonitor().DefaultInterfaceIndex(remoteAddr)
			}
		})
		dialer.Control = control.Append(dialer.Control, bindFunc)
		listener.Control = control.Append(listener.Control, bindFunc)
	} else if router.DefaultInterface() != "" {
		bindFunc := control.BindToInterface(router.InterfaceFinder(), router.DefaultInterface(), -1)
		dialer.Control = control.Append(dialer.Control, bindFunc)
		listener.Control = control.Append(listener.Control, bindFunc)
	}
	if options.RoutingMark != 0 {
		warnRoutingMarkOnUnsupportedPlatform.Check()
		dialer.Control = control.Append(dialer.Control, control.RoutingMark(options.RoutingMark))
		listener.Control = control.Append(listener.Control, control.RoutingMark(options.RoutingMark))
	} else if router.DefaultMark() != 0 {
		dialer.Control = control.Append(dialer.Control, control.RoutingMark(router.DefaultMark()))
		listener.Control = control.Append(listener.Control, control.RoutingMark(router.DefaultMark()))
	}
	if options.ReuseAddr {
		warnReuseAdderOnUnsupportedPlatform.Check()
		listener.Control = control.Append(listener.Control, control.ReuseAddr())
	}
	if options.ProtectPath != "" {
		warnProtectPathOnNonAndroid.Check()
		dialer.Control = control.Append(dialer.Control, control.ProtectPath(options.ProtectPath))
		listener.Control = control.Append(listener.Control, control.ProtectPath(options.ProtectPath))
	}
	if options.ConnectTimeout != 0 {
		dialer.Timeout = time.Duration(options.ConnectTimeout)
	} else {
		dialer.Timeout = C.TCPTimeout
	}
	if options.TCPFastOpen {
		warnTFOOnUnsupportedPlatform.Check()
	}
	var udpFragment bool
	if options.UDPFragment != nil {
		udpFragment = *options.UDPFragment
	} else {
		udpFragment = options.UDPFragmentDefault
	}
	if !udpFragment {
		dialer.Control = control.Append(dialer.Control, control.DisableUDPFragment())
		listener.Control = control.Append(listener.Control, control.DisableUDPFragment())
	}
	var bindUDPAddr string
	udpDialer := dialer
	var bindAddress netip.Addr
	if options.BindAddress != nil {
		bindAddress = options.BindAddress.Build()
	}
	if bindAddress.IsValid() {
		dialer.LocalAddr = &net.TCPAddr{
			IP: bindAddress.AsSlice(),
		}
		udpDialer.LocalAddr = &net.UDPAddr{
			IP: bindAddress.AsSlice(),
		}
		bindUDPAddr = M.SocksaddrFrom(bindAddress, 0).String()
	}
	return &DefaultDialer{tfo.Dialer{Dialer: dialer, DisableTFO: !options.TCPFastOpen}, udpDialer, listener, bindUDPAddr}
}

func (d *DefaultDialer) DialContext(ctx context.Context, network string, address M.Socksaddr) (net.Conn, error) {
	if !address.IsValid() {
		return nil, E.New("invalid address")
	}
	switch N.NetworkName(network) {
	case N.NetworkUDP:
		return d.udpDialer.DialContext(ctx, network, address.String())
	}
	return d.dialer.DialContext(ctx, network, address.Unwrap().String())
}

func (d *DefaultDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return d.udpListener.ListenPacket(ctx, N.NetworkUDP, d.bindUDPAddr)
}
