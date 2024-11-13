package dialer

import (
	"context"
	"net"
	"net/netip"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/conntrack"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/atomic"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var (
	_ ParallelInterfaceDialer = (*DefaultDialer)(nil)
	_ WireGuardListener       = (*DefaultDialer)(nil)
)

type DefaultDialer struct {
	dialer4              tcpDialer
	dialer6              tcpDialer
	udpDialer4           net.Dialer
	udpDialer6           net.Dialer
	udpListener          net.ListenConfig
	udpAddr4             string
	udpAddr6             string
	isWireGuardListener  bool
	networkManager       adapter.NetworkManager
	networkStrategy      C.NetworkStrategy
	networkType          []C.InterfaceType
	fallbackNetworkType  []C.InterfaceType
	networkFallbackDelay time.Duration
	networkLastFallback  atomic.TypedValue[time.Time]
}

func NewDefault(networkManager adapter.NetworkManager, options option.DialerOptions) (*DefaultDialer, error) {
	var (
		dialer               net.Dialer
		listener             net.ListenConfig
		interfaceFinder      control.InterfaceFinder
		networkStrategy      C.NetworkStrategy
		networkType          []C.InterfaceType
		fallbackNetworkType  []C.InterfaceType
		networkFallbackDelay time.Duration
	)
	if networkManager != nil {
		interfaceFinder = networkManager.InterfaceFinder()
	} else {
		interfaceFinder = control.NewDefaultInterfaceFinder()
	}
	if options.BindInterface != "" {
		bindFunc := control.BindToInterface(interfaceFinder, options.BindInterface, -1)
		dialer.Control = control.Append(dialer.Control, bindFunc)
		listener.Control = control.Append(listener.Control, bindFunc)
	}
	if options.RoutingMark > 0 {
		dialer.Control = control.Append(dialer.Control, control.RoutingMark(uint32(options.RoutingMark)))
		listener.Control = control.Append(listener.Control, control.RoutingMark(uint32(options.RoutingMark)))
	}
	if networkManager != nil {
		autoRedirectOutputMark := networkManager.AutoRedirectOutputMark()
		if autoRedirectOutputMark > 0 {
			if options.RoutingMark > 0 {
				return nil, E.New("`routing_mark` is conflict with `tun.auto_redirect` with `tun.route_[_exclude]_address_set")
			}
			dialer.Control = control.Append(dialer.Control, control.RoutingMark(autoRedirectOutputMark))
			listener.Control = control.Append(listener.Control, control.RoutingMark(autoRedirectOutputMark))
		}
	}
	if C.NetworkStrategy(options.NetworkStrategy) != C.NetworkStrategyDefault {
		if options.BindInterface != "" || options.Inet4BindAddress != nil || options.Inet6BindAddress != nil {
			return nil, E.New("`network_strategy` is conflict with `bind_interface`, `inet4_bind_address` and `inet6_bind_address`")
		}
		networkStrategy = C.NetworkStrategy(options.NetworkStrategy)
		networkType = common.Map(options.NetworkType, option.InterfaceType.Build)
		fallbackNetworkType = common.Map(options.FallbackNetworkType, option.InterfaceType.Build)
		networkFallbackDelay = time.Duration(options.NetworkFallbackDelay)
		if networkManager == nil || !networkManager.AutoDetectInterface() {
			return nil, E.New("`route.auto_detect_interface` is require by `network_strategy`")
		}
	}
	if networkManager != nil && options.BindInterface == "" && options.Inet4BindAddress == nil && options.Inet6BindAddress == nil {
		defaultOptions := networkManager.DefaultOptions()
		if options.BindInterface == "" {
			if defaultOptions.BindInterface != "" {
				bindFunc := control.BindToInterface(networkManager.InterfaceFinder(), defaultOptions.BindInterface, -1)
				dialer.Control = control.Append(dialer.Control, bindFunc)
				listener.Control = control.Append(listener.Control, bindFunc)
			} else if networkManager.AutoDetectInterface() {
				if defaultOptions.NetworkStrategy != C.NetworkStrategyDefault && C.NetworkStrategy(options.NetworkStrategy) == C.NetworkStrategyDefault {
					networkStrategy = defaultOptions.NetworkStrategy
					networkType = defaultOptions.NetworkType
					fallbackNetworkType = defaultOptions.FallbackNetworkType
					networkFallbackDelay = defaultOptions.FallbackDelay
					bindFunc := networkManager.ProtectFunc()
					dialer.Control = control.Append(dialer.Control, bindFunc)
					listener.Control = control.Append(listener.Control, bindFunc)
				} else {
					bindFunc := networkManager.AutoDetectInterfaceFunc()
					dialer.Control = control.Append(dialer.Control, bindFunc)
					listener.Control = control.Append(listener.Control, bindFunc)
				}
			}
		}
		if options.RoutingMark == 0 && defaultOptions.RoutingMark != 0 {
			dialer.Control = control.Append(dialer.Control, control.RoutingMark(defaultOptions.RoutingMark))
			listener.Control = control.Append(listener.Control, control.RoutingMark(defaultOptions.RoutingMark))
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
		bindAddr := options.Inet4BindAddress.Build(netip.IPv4Unspecified())
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
		bindAddr := options.Inet6BindAddress.Build(netip.IPv6Unspecified())
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
	if networkStrategy != C.NetworkStrategyDefault && options.TCPFastOpen {
		return nil, E.New("`tcp_fast_open` is conflict with `network_strategy` or `route.default_network_strategy`")
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
		dialer4:              tcpDialer4,
		dialer6:              tcpDialer6,
		udpDialer4:           udpDialer4,
		udpDialer6:           udpDialer6,
		udpListener:          listener,
		udpAddr4:             udpAddr4,
		udpAddr6:             udpAddr6,
		isWireGuardListener:  options.IsWireGuardListener,
		networkManager:       networkManager,
		networkStrategy:      networkStrategy,
		networkType:          networkType,
		fallbackNetworkType:  fallbackNetworkType,
		networkFallbackDelay: networkFallbackDelay,
	}, nil
}

func (d *DefaultDialer) DialContext(ctx context.Context, network string, address M.Socksaddr) (net.Conn, error) {
	if !address.IsValid() {
		return nil, E.New("invalid address")
	}
	if d.networkStrategy == C.NetworkStrategyDefault {
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
	} else {
		return d.DialParallelInterface(ctx, network, address, d.networkStrategy, d.networkType, d.fallbackNetworkType, d.networkFallbackDelay)
	}
}

func (d *DefaultDialer) DialParallelInterface(ctx context.Context, network string, address M.Socksaddr, strategy C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.Conn, error) {
	if strategy == C.NetworkStrategyDefault {
		return d.DialContext(ctx, network, address)
	}
	if !d.networkManager.AutoDetectInterface() {
		return nil, E.New("`route.auto_detect_interface` is require by `network_strategy`")
	}
	var dialer net.Dialer
	if N.NetworkName(network) == N.NetworkTCP {
		dialer = dialerFromTCPDialer(d.dialer4)
	} else {
		dialer = d.udpDialer4
	}
	fastFallback := time.Now().Sub(d.networkLastFallback.Load()) < C.TCPTimeout
	var (
		conn      net.Conn
		isPrimary bool
		err       error
	)
	if !fastFallback {
		conn, isPrimary, err = d.dialParallelInterface(ctx, dialer, network, address.String(), strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
	} else {
		conn, isPrimary, err = d.dialParallelInterfaceFastFallback(ctx, dialer, network, address.String(), strategy, interfaceType, fallbackInterfaceType, fallbackDelay, d.networkLastFallback.Store)
	}
	if err != nil {
		return nil, err
	}
	if !fastFallback && !isPrimary {
		d.networkLastFallback.Store(time.Now())
	}
	return trackConn(conn, nil)
}

func (d *DefaultDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	if d.networkStrategy == C.NetworkStrategyDefault {
		if destination.IsIPv6() {
			return trackPacketConn(d.udpListener.ListenPacket(ctx, N.NetworkUDP, d.udpAddr6))
		} else if destination.IsIPv4() && !destination.Addr.IsUnspecified() {
			return trackPacketConn(d.udpListener.ListenPacket(ctx, N.NetworkUDP+"4", d.udpAddr4))
		} else {
			return trackPacketConn(d.udpListener.ListenPacket(ctx, N.NetworkUDP, d.udpAddr4))
		}
	} else {
		return d.ListenSerialInterfacePacket(ctx, destination, d.networkStrategy, d.networkType, d.fallbackNetworkType, d.networkFallbackDelay)
	}
}

func (d *DefaultDialer) ListenSerialInterfacePacket(ctx context.Context, destination M.Socksaddr, strategy C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.PacketConn, error) {
	if strategy == C.NetworkStrategyDefault {
		return d.ListenPacket(ctx, destination)
	}
	if !d.networkManager.AutoDetectInterface() {
		return nil, E.New("`route.auto_detect_interface` is require by `network_strategy`")
	}
	network := N.NetworkUDP
	if destination.IsIPv4() && !destination.Addr.IsUnspecified() {
		network += "4"
	}
	return trackPacketConn(d.listenSerialInterfacePacket(ctx, d.udpListener, network, "", strategy, interfaceType, fallbackInterfaceType, fallbackDelay))
}

func (d *DefaultDialer) ListenPacketCompat(network, address string) (net.PacketConn, error) {
	return d.listenSerialInterfacePacket(context.Background(), d.udpListener, network, address, d.networkStrategy, d.networkType, d.fallbackNetworkType, d.networkFallbackDelay)
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
