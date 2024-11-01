package dialer

import (
	"context"
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/conntrack"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ WireGuardListener = (*DefaultDialer)(nil)

type DefaultDialer struct {
	dialer4             tcpDialer
	dialer6             tcpDialer
	udpDialer4          net.Dialer
	udpDialer6          net.Dialer
	udpListener         net.ListenConfig
	udpAddr4            string
	udpAddr6            string
	isWireGuardListener bool
}

func NewDefault(router adapter.Router, options option.DialerOptions) (*DefaultDialer, error) {
	var dialer net.Dialer
	var listener net.ListenConfig
	if options.BindInterface != "" {
		var interfaceFinder control.InterfaceFinder
		if router != nil {
			interfaceFinder = router.InterfaceFinder()
		} else {
			interfaceFinder = control.NewDefaultInterfaceFinder()
		}
		bindFunc := control.BindToInterface(interfaceFinder, options.BindInterface, -1)
		dialer.Control = control.Append(dialer.Control, bindFunc)
		listener.Control = control.Append(listener.Control, bindFunc)
	} else if router != nil && router.AutoDetectInterface() {
		bindFunc := router.AutoDetectInterfaceFunc()
		dialer.Control = control.Append(dialer.Control, bindFunc)
		listener.Control = control.Append(listener.Control, bindFunc)
	} else if router != nil && router.DefaultInterface() != "" {
		bindFunc := control.BindToInterface(router.InterfaceFinder(), router.DefaultInterface(), -1)
		dialer.Control = control.Append(dialer.Control, bindFunc)
		listener.Control = control.Append(listener.Control, bindFunc)
	}
	var autoRedirectOutputMark uint32
	if router != nil {
		autoRedirectOutputMark = router.AutoRedirectOutputMark()
	}
	if autoRedirectOutputMark > 0 {
		dialer.Control = control.Append(dialer.Control, control.RoutingMark(autoRedirectOutputMark))
		listener.Control = control.Append(listener.Control, control.RoutingMark(autoRedirectOutputMark))
	}
	if options.RoutingMark > 0 {
		dialer.Control = control.Append(dialer.Control, control.RoutingMark(options.RoutingMark))
		listener.Control = control.Append(listener.Control, control.RoutingMark(options.RoutingMark))
		if autoRedirectOutputMark > 0 {
			return nil, E.New("`auto_redirect` with `route_[_exclude]_address_set is conflict with `routing_mark`")
		}
	} else if router != nil && router.DefaultMark() > 0 {
		dialer.Control = control.Append(dialer.Control, control.RoutingMark(router.DefaultMark()))
		listener.Control = control.Append(listener.Control, control.RoutingMark(router.DefaultMark()))
		if autoRedirectOutputMark > 0 {
			return nil, E.New("`auto_redirect` with `route_[_exclude]_address_set is conflict with `default_mark`")
		}
	}
	if options.ReuseAddr {
		listener.Control = control.Append(listener.Control, control.ReuseAddr())
	}
	if options.ProtectPath != "" {
		dialer.Control = control.Append(dialer.Control, control.ProtectPath(options.ProtectPath))
		listener.Control = control.Append(listener.Control, control.ProtectPath(options.ProtectPath))
	}
	if options.ConnectTimeout != 0 {
		dialer.Timeout = time.Duration(options.ConnectTimeout)
	} else {
		dialer.Timeout = C.TCPConnectTimeout
	}
	// TODO: Add an option to customize the keep alive period
	dialer.KeepAlive = C.TCPKeepAliveInitial
	dialer.Control = control.Append(dialer.Control, control.SetKeepAlivePeriod(C.TCPKeepAliveInitial, C.TCPKeepAliveInterval))
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
	if options.TCPMultiPath {
		if !go121Available {
			return nil, E.New("MultiPath TCP requires go1.21, please recompile your binary.")
		}
		setMultiPathTCP(&dialer4)
	}
	if options.IsWireGuardListener {
		for _, controlFn := range WgControlFns {
			listener.Control = control.Append(listener.Control, controlFn)
		}
	}
	tcpDialer4, err := newTCPDialer(dialer4, options.TCPFastOpen)
	if err != nil {
		return nil, err
	}
	tcpDialer6, err := newTCPDialer(dialer6, options.TCPFastOpen)
	if err != nil {
		return nil, err
	}
	return &DefaultDialer{
		tcpDialer4,
		tcpDialer6,
		udpDialer4,
		udpDialer6,
		listener,
		udpAddr4,
		udpAddr6,
		options.IsWireGuardListener,
	}, nil
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
	if destination.IsIPv6() {
		return trackPacketConn(d.udpListener.ListenPacket(ctx, N.NetworkUDP, d.udpAddr6))
	} else if destination.IsIPv4() && !destination.Addr.IsUnspecified() {
		return trackPacketConn(d.udpListener.ListenPacket(ctx, N.NetworkUDP+"4", d.udpAddr4))
	} else {
		return trackPacketConn(d.udpListener.ListenPacket(ctx, N.NetworkUDP, d.udpAddr4))
	}
}

func (d *DefaultDialer) ListenPacketCompat(network, address string) (net.PacketConn, error) {
	return d.udpListener.ListenPacket(context.Background(), network, address)
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
