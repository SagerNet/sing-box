package dialer

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"syscall"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/conntrack"
	"github.com/sagernet/sing-box/common/listener"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/libbox/platform"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"

	"github.com/database64128/tfo-go/v2"
)

var (
	_ ParallelInterfaceDialer = (*DefaultDialer)(nil)
	_ WireGuardListener       = (*DefaultDialer)(nil)
)

type DefaultDialer struct {
	dialer4                tfo.Dialer
	dialer6                tfo.Dialer
	udpDialer4             net.Dialer
	udpDialer6             net.Dialer
	udpListener            net.ListenConfig
	udpAddr4               string
	udpAddr6               string
	netns                  string
	networkManager         adapter.NetworkManager
	networkStrategy        *C.NetworkStrategy
	defaultNetworkStrategy bool
	networkType            []C.InterfaceType
	fallbackNetworkType    []C.InterfaceType
	networkFallbackDelay   time.Duration
	networkLastFallback    common.TypedValue[time.Time]
}

func NewDefault(ctx context.Context, options option.DialerOptions) (*DefaultDialer, error) {
	networkManager := service.FromContext[adapter.NetworkManager](ctx)
	platformInterface := service.FromContext[platform.Interface](ctx)

	var (
		dialer                 net.Dialer
		listener               net.ListenConfig
		interfaceFinder        control.InterfaceFinder
		networkStrategy        *C.NetworkStrategy
		defaultNetworkStrategy bool
		networkType            []C.InterfaceType
		fallbackNetworkType    []C.InterfaceType
		networkFallbackDelay   time.Duration
	)
	if networkManager != nil {
		interfaceFinder = networkManager.InterfaceFinder()
	} else {
		interfaceFinder = control.NewDefaultInterfaceFinder()
	}
	if options.BindInterface != "" {
		if !(C.IsLinux || C.IsDarwin || C.IsWindows) {
			return nil, E.New("`bind_interface` is only supported on Linux, macOS and Windows")
		}
		bindFunc := control.BindToInterface(interfaceFinder, options.BindInterface, -1)
		dialer.Control = control.Append(dialer.Control, bindFunc)
		listener.Control = control.Append(listener.Control, bindFunc)
	}
	if options.RoutingMark > 0 {
		if !C.IsLinux {
			return nil, E.New("`routing_mark` is only supported on Linux")
		}
		dialer.Control = control.Append(dialer.Control, setMarkWrapper(networkManager, uint32(options.RoutingMark), false))
		listener.Control = control.Append(listener.Control, setMarkWrapper(networkManager, uint32(options.RoutingMark), false))
	}
	disableDefaultBind := options.BindInterface != "" || options.Inet4BindAddress != nil || options.Inet6BindAddress != nil
	if disableDefaultBind || options.TCPFastOpen {
		if options.NetworkStrategy != nil || len(options.NetworkType) > 0 && options.FallbackNetworkType == nil && options.FallbackDelay == 0 {
			return nil, E.New("`network_strategy` is conflict with `bind_interface`, `inet4_bind_address`, `inet6_bind_address` and `tcp_fast_open`")
		}
	}

	if networkManager != nil {
		defaultOptions := networkManager.DefaultOptions()
		if defaultOptions.BindInterface != "" {
			bindFunc := control.BindToInterface(networkManager.InterfaceFinder(), defaultOptions.BindInterface, -1)
			dialer.Control = control.Append(dialer.Control, bindFunc)
			listener.Control = control.Append(listener.Control, bindFunc)
		} else if networkManager.AutoDetectInterface() && !disableDefaultBind {
			if platformInterface != nil {
				networkStrategy = (*C.NetworkStrategy)(options.NetworkStrategy)
				networkType = common.Map(options.NetworkType, option.InterfaceType.Build)
				fallbackNetworkType = common.Map(options.FallbackNetworkType, option.InterfaceType.Build)
				if networkStrategy == nil && len(networkType) == 0 && len(fallbackNetworkType) == 0 {
					networkStrategy = defaultOptions.NetworkStrategy
					networkType = defaultOptions.NetworkType
					fallbackNetworkType = defaultOptions.FallbackNetworkType
				}
				networkFallbackDelay = time.Duration(options.FallbackDelay)
				if networkFallbackDelay == 0 && defaultOptions.FallbackDelay != 0 {
					networkFallbackDelay = defaultOptions.FallbackDelay
				}
				if networkStrategy == nil {
					networkStrategy = common.Ptr(C.NetworkStrategyDefault)
					defaultNetworkStrategy = true
				}
				bindFunc := networkManager.ProtectFunc()
				dialer.Control = control.Append(dialer.Control, bindFunc)
				listener.Control = control.Append(listener.Control, bindFunc)
			} else {
				bindFunc := networkManager.AutoDetectInterfaceFunc()
				dialer.Control = control.Append(dialer.Control, bindFunc)
				listener.Control = control.Append(listener.Control, bindFunc)
			}
		}
		if options.RoutingMark == 0 && defaultOptions.RoutingMark != 0 {
			dialer.Control = control.Append(dialer.Control, setMarkWrapper(networkManager, defaultOptions.RoutingMark, true))
			listener.Control = control.Append(listener.Control, setMarkWrapper(networkManager, defaultOptions.RoutingMark, true))
		}
	}
	if networkManager != nil {
		markFunc := networkManager.AutoRedirectOutputMarkFunc()
		dialer.Control = control.Append(dialer.Control, markFunc)
		listener.Control = control.Append(listener.Control, markFunc)
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
		dialer4.SetMultipathTCP(true)
	}
	tcpDialer4 := tfo.Dialer{Dialer: dialer4, DisableTFO: !options.TCPFastOpen}
	tcpDialer6 := tfo.Dialer{Dialer: dialer6, DisableTFO: !options.TCPFastOpen}
	return &DefaultDialer{
		dialer4:                tcpDialer4,
		dialer6:                tcpDialer6,
		udpDialer4:             udpDialer4,
		udpDialer6:             udpDialer6,
		udpListener:            listener,
		udpAddr4:               udpAddr4,
		udpAddr6:               udpAddr6,
		netns:                  options.NetNs,
		networkManager:         networkManager,
		networkStrategy:        networkStrategy,
		defaultNetworkStrategy: defaultNetworkStrategy,
		networkType:            networkType,
		fallbackNetworkType:    fallbackNetworkType,
		networkFallbackDelay:   networkFallbackDelay,
	}, nil
}

func setMarkWrapper(networkManager adapter.NetworkManager, mark uint32, isDefault bool) control.Func {
	if networkManager == nil {
		return control.RoutingMark(mark)
	}
	return func(network, address string, conn syscall.RawConn) error {
		if networkManager.AutoRedirectOutputMark() != 0 {
			if isDefault {
				return E.New("`route.default_mark` is conflict with `tun.auto_redirect`")
			} else {
				return E.New("`routing_mark` is conflict with `tun.auto_redirect`")
			}
		}
		return control.RoutingMark(mark)(network, address, conn)
	}
}

func (d *DefaultDialer) DialContext(ctx context.Context, network string, address M.Socksaddr) (net.Conn, error) {
	if !address.IsValid() {
		return nil, E.New("invalid address")
	} else if address.IsFqdn() {
		return nil, E.New("domain not resolved")
	}
	if d.networkStrategy == nil {
		return trackConn(listener.ListenNetworkNamespace[net.Conn](d.netns, func() (net.Conn, error) {
			switch N.NetworkName(network) {
			case N.NetworkUDP:
				if !address.IsIPv6() {
					return d.udpDialer4.DialContext(ctx, network, address.String())
				} else {
					return d.udpDialer6.DialContext(ctx, network, address.String())
				}
			}
			if !address.IsIPv6() {
				return DialSlowContext(&d.dialer4, ctx, network, address)
			} else {
				return DialSlowContext(&d.dialer6, ctx, network, address)
			}
		}))
	} else {
		return d.DialParallelInterface(ctx, network, address, d.networkStrategy, d.networkType, d.fallbackNetworkType, d.networkFallbackDelay)
	}
}

func (d *DefaultDialer) DialParallelInterface(ctx context.Context, network string, address M.Socksaddr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.Conn, error) {
	if strategy == nil {
		strategy = d.networkStrategy
	}
	if strategy == nil {
		return d.DialContext(ctx, network, address)
	}
	if len(interfaceType) == 0 {
		interfaceType = d.networkType
	}
	if len(fallbackInterfaceType) == 0 {
		fallbackInterfaceType = d.fallbackNetworkType
	}
	if fallbackDelay == 0 {
		fallbackDelay = d.networkFallbackDelay
	}
	var dialer net.Dialer
	if N.NetworkName(network) == N.NetworkTCP {
		dialer = d.dialer4.Dialer
	} else {
		dialer = d.udpDialer4
	}
	fastFallback := time.Since(d.networkLastFallback.Load()) < C.TCPTimeout
	var (
		conn      net.Conn
		isPrimary bool
		err       error
	)
	if !fastFallback {
		conn, isPrimary, err = d.dialParallelInterface(ctx, dialer, network, address.String(), *strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
	} else {
		conn, isPrimary, err = d.dialParallelInterfaceFastFallback(ctx, dialer, network, address.String(), *strategy, interfaceType, fallbackInterfaceType, fallbackDelay, d.networkLastFallback.Store)
	}
	if err != nil {
		// bind interface failed on legacy xiaomi systems
		if d.defaultNetworkStrategy && errors.Is(err, syscall.EPERM) {
			d.networkStrategy = nil
			return d.DialContext(ctx, network, address)
		} else {
			return nil, err
		}
	}
	if !fastFallback && !isPrimary {
		d.networkLastFallback.Store(time.Now())
	}
	return trackConn(conn, nil)
}

func (d *DefaultDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	if d.networkStrategy == nil {
		return trackPacketConn(listener.ListenNetworkNamespace[net.PacketConn](d.netns, func() (net.PacketConn, error) {
			if destination.IsIPv6() {
				return d.udpListener.ListenPacket(ctx, N.NetworkUDP, d.udpAddr6)
			} else if destination.IsIPv4() && !destination.Addr.IsUnspecified() {
				return d.udpListener.ListenPacket(ctx, N.NetworkUDP+"4", d.udpAddr4)
			} else {
				return d.udpListener.ListenPacket(ctx, N.NetworkUDP, d.udpAddr4)
			}
		}))
	} else {
		return d.ListenSerialInterfacePacket(ctx, destination, d.networkStrategy, d.networkType, d.fallbackNetworkType, d.networkFallbackDelay)
	}
}

func (d *DefaultDialer) DialerForICMPDestination(destination netip.Addr) net.Dialer {
	if !destination.Is6() {
		return d.dialer6.Dialer
	} else {
		return d.dialer4.Dialer
	}
}

func (d *DefaultDialer) ListenSerialInterfacePacket(ctx context.Context, destination M.Socksaddr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.PacketConn, error) {
	if strategy == nil {
		strategy = d.networkStrategy
	}
	if strategy == nil {
		return d.ListenPacket(ctx, destination)
	}
	if len(interfaceType) == 0 {
		interfaceType = d.networkType
	}
	if len(fallbackInterfaceType) == 0 {
		fallbackInterfaceType = d.fallbackNetworkType
	}
	if fallbackDelay == 0 {
		fallbackDelay = d.networkFallbackDelay
	}
	network := N.NetworkUDP
	if destination.IsIPv4() && !destination.Addr.IsUnspecified() {
		network += "4"
	}
	packetConn, err := d.listenSerialInterfacePacket(ctx, d.udpListener, network, "", *strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
	if err != nil {
		// bind interface failed on legacy xiaomi systems
		if d.defaultNetworkStrategy && errors.Is(err, syscall.EPERM) {
			d.networkStrategy = nil
			return d.ListenPacket(ctx, destination)
		} else {
			return nil, err
		}
	}
	return trackPacketConn(packetConn, nil)
}

func (d *DefaultDialer) WireGuardControl() control.Func {
	return d.udpListener.Control
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
