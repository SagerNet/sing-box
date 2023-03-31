package dialer

import (
	"context"
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer/conntrack"
	"github.com/sagernet/sing-box/common/warning"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/tfo-go"
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
	dialer4     tfo.Dialer
	dialer6     tfo.Dialer
	udpDialer4  net.Dialer
	udpDialer6  net.Dialer
	udpListener net.ListenConfig
	udpAddr4    string
	udpAddr6    string
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
		bindFunc := router.AutoDetectInterfaceFunc()
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
	var (
		dialer4    = dialer
		udpDialer4 = dialer
		udpAddr4   string
	)
	if options.Inet4BindAddress != nil {
		bindAddr := options.Inet4BindAddress.Build()
		dialer4.LocalAddr = &net.TCPAddr{IP: bindAddr.AsSlice()}
		udpDialer4.LocalAddr = &net.UDPAddr{IP: bindAddr.AsSlice()}
		udpAddr4 = M.SocksaddrFrom(bindAddr, 0).String()
	}
	var (
		dialer6    = dialer
		udpDialer6 = dialer
		udpAddr6   string
	)
	if options.Inet6BindAddress != nil {
		bindAddr := options.Inet6BindAddress.Build()
		dialer6.LocalAddr = &net.TCPAddr{IP: bindAddr.AsSlice()}
		udpDialer6.LocalAddr = &net.UDPAddr{IP: bindAddr.AsSlice()}
		udpAddr6 = M.SocksaddrFrom(bindAddr, 0).String()
	}
	return &DefaultDialer{
		tfo.Dialer{Dialer: dialer4, DisableTFO: !options.TCPFastOpen},
		tfo.Dialer{Dialer: dialer6, DisableTFO: !options.TCPFastOpen},
		udpDialer4,
		udpDialer6,
		listener,
		udpAddr4,
		udpAddr6,
	}
}

func (d *DefaultDialer) DialContext(ctx context.Context, network string, address M.Socksaddr) (net.Conn, error) {
	if !address.IsValid() {
		return nil, E.New("invalid address")
	}
	switch N.NetworkName(network) {
	case N.NetworkUDP:
		if !address.IsIPv6() {
			return trackConn(d.udpDialer4.DialContext(ctx, network, address.String()))
		} else {
			return trackConn(d.udpDialer6.DialContext(ctx, network, address.String()))
		}
	}
	if !address.IsIPv6() {
		return trackConn(DialSlowContext(&d.dialer4, ctx, network, address))
	} else {
		return trackConn(DialSlowContext(&d.dialer6, ctx, network, address))
	}
}

func (d *DefaultDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	if !destination.IsIPv6() {
		return trackPacketConn(d.udpListener.ListenPacket(ctx, N.NetworkUDP, d.udpAddr4))
	} else {
		return trackPacketConn(d.udpListener.ListenPacket(ctx, N.NetworkUDP, d.udpAddr6))
	}
}

func trackConn(conn net.Conn, err error) (net.Conn, error) {
	if !conntrack.Enabled || err != nil {
		return conn, err
	}
	return conntrack.NewConn(conn)
}

func trackPacketConn(conn net.PacketConn, err error) (net.PacketConn, error) {
	if !conntrack.Enabled || err != nil {
		return conn, err
	}
	return conntrack.NewPacketConn(conn)
}
