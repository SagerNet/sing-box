package tailscale

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/sagernet/gvisor/pkg/tcpip"
	"github.com/sagernet/gvisor/pkg/tcpip/adapters/gonet"
	"github.com/sagernet/gvisor/pkg/tcpip/header"
	"github.com/sagernet/gvisor/pkg/tcpip/stack"
	"github.com/sagernet/gvisor/pkg/tcpip/transport/icmp"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/endpoint"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/route/rule"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing-tun/ping"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/ntp"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/filemanager"
	_ "github.com/sagernet/tailscale/feature/relayserver"
	"github.com/sagernet/tailscale/ipn"
	tsDNS "github.com/sagernet/tailscale/net/dns"
	"github.com/sagernet/tailscale/net/netmon"
	"github.com/sagernet/tailscale/net/tsaddr"
	tsTUN "github.com/sagernet/tailscale/net/tstun"
	"github.com/sagernet/tailscale/tsnet"
	"github.com/sagernet/tailscale/types/ipproto"
	"github.com/sagernet/tailscale/types/nettype"
	"github.com/sagernet/tailscale/version"
	"github.com/sagernet/tailscale/wgengine"
	"github.com/sagernet/tailscale/wgengine/filter"
	"github.com/sagernet/tailscale/wgengine/router"
	"github.com/sagernet/tailscale/wgengine/wgcfg"

	"go4.org/netipx"
)

var (
	_ adapter.OutboundWithPreferredRoutes = (*Endpoint)(nil)
	_ adapter.DirectRouteOutbound         = (*Endpoint)(nil)
)

func init() {
	version.SetVersion("sing-box " + C.Version)
}

func RegisterEndpoint(registry *endpoint.Registry) {
	endpoint.Register[option.TailscaleEndpointOptions](registry, C.TypeTailscale, NewEndpoint)
}

type Endpoint struct {
	endpoint.Adapter
	ctx               context.Context
	router            adapter.Router
	logger            logger.ContextLogger
	dnsRouter         adapter.DNSRouter
	network           adapter.NetworkManager
	platformInterface adapter.PlatformInterface
	server            *tsnet.Server
	stack             *stack.Stack
	icmpForwarder     *tun.ICMPForwarder
	filter            *atomic.Pointer[filter.Filter]
	onReconfigHook    wgengine.ReconfigListener

	cfg           *wgcfg.Config
	dnsCfg        *tsDNS.Config
	routeDomains  common.TypedValue[map[string]bool]
	routePrefixes atomic.Pointer[netipx.IPSet]

	acceptRoutes               bool
	exitNode                   string
	exitNodeAllowLANAccess     bool
	advertiseRoutes            []netip.Prefix
	advertiseExitNode          bool
	relayServerPort            *uint16
	relayServerStaticEndpoints []netip.AddrPort

	udpTimeout time.Duration

	systemInterface     bool
	systemInterfaceName string
	systemInterfaceMTU  uint32
	systemTun           tun.Tun
	fallbackTCPCloser   func()
}

func (t *Endpoint) registerNetstackHandlers() {
	netstack := t.server.ExportNetstack()
	if netstack == nil {
		return
	}
	previousTCP := netstack.GetTCPHandlerForFlow
	netstack.GetTCPHandlerForFlow = func(src, dst netip.AddrPort) (handler func(net.Conn), intercept bool) {
		if previousTCP != nil {
			handler, intercept = previousTCP(src, dst)
			if handler != nil || !intercept {
				return handler, intercept
			}
		}
		return func(conn net.Conn) {
			ctx := log.ContextWithNewID(t.ctx)
			source := M.SocksaddrFrom(src.Addr(), src.Port())
			destination := M.SocksaddrFrom(dst.Addr(), dst.Port())
			t.NewConnectionEx(ctx, conn, source, destination, nil)
		}, true
	}

	previousUDP := netstack.GetUDPHandlerForFlow
	netstack.GetUDPHandlerForFlow = func(src, dst netip.AddrPort) (handler func(nettype.ConnPacketConn), intercept bool) {
		if previousUDP != nil {
			handler, intercept = previousUDP(src, dst)
			if handler != nil || !intercept {
				return handler, intercept
			}
		}
		return func(conn nettype.ConnPacketConn) {
			ctx := log.ContextWithNewID(t.ctx)
			source := M.SocksaddrFrom(src.Addr(), src.Port())
			destination := M.SocksaddrFrom(dst.Addr(), dst.Port())
			packetConn := bufio.NewPacketConn(conn)
			t.NewPacketConnectionEx(ctx, packetConn, source, destination, nil)
		}, true
	}
}

func NewEndpoint(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.TailscaleEndpointOptions) (adapter.Endpoint, error) {
	stateDirectory := options.StateDirectory
	if stateDirectory == "" {
		stateDirectory = "tailscale"
	}
	hostname := options.Hostname
	if hostname == "" {
		osHostname, _ := os.Hostname()
		osHostname = strings.TrimSpace(osHostname)
		hostname = osHostname
	}
	if hostname == "" {
		hostname = "sing-box"
	}
	stateDirectory = filemanager.BasePath(ctx, os.ExpandEnv(stateDirectory))
	stateDirectory, _ = filepath.Abs(stateDirectory)
	for _, advertiseRoute := range options.AdvertiseRoutes {
		if advertiseRoute.Addr().IsUnspecified() && advertiseRoute.Bits() == 0 {
			return nil, E.New("`advertise_routes` cannot be default, use `advertise_exit_node` instead.")
		}
	}
	if options.AdvertiseExitNode && options.ExitNode != "" {
		return nil, E.New("cannot advertise an exit node and use an exit node at the same time.")
	}
	var udpTimeout time.Duration
	if options.UDPTimeout != 0 {
		udpTimeout = time.Duration(options.UDPTimeout)
	} else {
		udpTimeout = C.UDPTimeout
	}
	var remoteIsDomain bool
	if options.ControlURL != "" {
		controlURL, err := url.Parse(options.ControlURL)
		if err != nil {
			return nil, E.Cause(err, "parse control URL")
		}
		remoteIsDomain = M.IsDomainName(controlURL.Hostname())
	} else {
		// controlplane.tailscale.com
		remoteIsDomain = true
	}
	outboundDialer, err := dialer.NewWithOptions(dialer.Options{
		Context:          ctx,
		Options:          options.DialerOptions,
		RemoteIsDomain:   remoteIsDomain,
		ResolverOnDetour: true,
		NewDialer:        true,
	})
	if err != nil {
		return nil, err
	}
	dnsRouter := service.FromContext[adapter.DNSRouter](ctx)
	server := &tsnet.Server{
		Dir:      stateDirectory,
		Hostname: hostname,
		Logf: func(format string, args ...any) {
			logger.Trace(fmt.Sprintf(format, args...))
		},
		UserLogf: func(format string, args ...any) {
			logger.Debug(fmt.Sprintf(format, args...))
		},
		Ephemeral:  options.Ephemeral,
		AuthKey:    options.AuthKey,
		ControlURL: options.ControlURL,
		Dialer:     &endpointDialer{Dialer: outboundDialer, logger: logger},
		LookupHook: func(ctx context.Context, host string) ([]netip.Addr, error) {
			return dnsRouter.Lookup(ctx, host, outboundDialer.(dialer.ResolveDialer).QueryOptions())
		},
		DNS: &dnsConfigurtor{},
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				ForceAttemptHTTP2: true,
				DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
					return outboundDialer.DialContext(ctx, network, M.ParseSocksaddr(address))
				},
				TLSClientConfig: &tls.Config{
					RootCAs: adapter.RootPoolFromContext(ctx),
					Time:    ntp.TimeFuncFromContext(ctx),
				},
			},
		},
	}
	return &Endpoint{
		Adapter:                    endpoint.NewAdapter(C.TypeTailscale, tag, []string{N.NetworkTCP, N.NetworkUDP, N.NetworkICMP}, nil),
		ctx:                        ctx,
		router:                     router,
		logger:                     logger,
		dnsRouter:                  dnsRouter,
		network:                    service.FromContext[adapter.NetworkManager](ctx),
		platformInterface:          service.FromContext[adapter.PlatformInterface](ctx),
		server:                     server,
		acceptRoutes:               options.AcceptRoutes,
		exitNode:                   options.ExitNode,
		exitNodeAllowLANAccess:     options.ExitNodeAllowLANAccess,
		advertiseRoutes:            options.AdvertiseRoutes,
		advertiseExitNode:          options.AdvertiseExitNode,
		relayServerPort:            options.RelayServerPort,
		relayServerStaticEndpoints: options.RelayServerStaticEndpoints,
		udpTimeout:                 udpTimeout,
		systemInterface:            options.SystemInterface,
		systemInterfaceName:        options.SystemInterfaceName,
		systemInterfaceMTU:         options.SystemInterfaceMTU,
	}, nil
}

func (t *Endpoint) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	if t.platformInterface != nil {
		err := t.network.UpdateInterfaces()
		if err != nil {
			return err
		}
		netmon.RegisterInterfaceGetter(func() ([]netmon.Interface, error) {
			return common.Map(t.network.InterfaceFinder().Interfaces(), func(it control.Interface) netmon.Interface {
				return netmon.Interface{
					Interface: &net.Interface{
						Index:        it.Index,
						MTU:          it.MTU,
						Name:         it.Name,
						HardwareAddr: it.HardwareAddr,
						Flags:        it.Flags,
					},
					AltAddrs: common.Map(it.Addresses, func(it netip.Prefix) net.Addr {
						return &net.IPNet{
							IP:   it.Addr().AsSlice(),
							Mask: net.CIDRMask(it.Bits(), it.Addr().BitLen()),
						}
					}),
				}
			}), nil
		})
		if runtime.GOOS == "android" {
			setAndroidProtectFunc(t.platformInterface)
		}
	}
	if t.systemInterface {
		mtu := t.systemInterfaceMTU
		if mtu == 0 {
			mtu = uint32(tsTUN.DefaultTUNMTU())
		}
		tunName := t.systemInterfaceName
		if tunName == "" {
			tunName = tun.CalculateInterfaceName("tailscale")
		}
		tunOptions := tun.Options{
			Name:                      tunName,
			MTU:                       mtu,
			GSO:                       true,
			InterfaceScope:            true,
			InterfaceMonitor:          t.network.InterfaceMonitor(),
			InterfaceFinder:           t.network.InterfaceFinder(),
			Logger:                    t.logger,
			EXP_ExternalConfiguration: true,
		}
		systemTun, err := tun.New(tunOptions)
		if err != nil {
			return err
		}
		err = systemTun.Start()
		if err != nil {
			_ = systemTun.Close()
			return err
		}
		wgTunDevice, err := newTunDeviceAdapter(systemTun, int(mtu), t.logger)
		if err != nil {
			_ = systemTun.Close()
			return err
		}
		t.systemTun = systemTun
		t.server.TunDevice = wgTunDevice
	}
	err := t.server.Start()
	if err != nil {
		if t.systemTun != nil {
			_ = t.systemTun.Close()
		}
		return err
	}
	if t.fallbackTCPCloser == nil {
		t.fallbackTCPCloser = t.server.RegisterFallbackTCPHandler(func(src, dst netip.AddrPort) (handler func(net.Conn), intercept bool) {
			return func(conn net.Conn) {
				ctx := log.ContextWithNewID(t.ctx)
				source := M.SocksaddrFrom(src.Addr(), src.Port())
				destination := M.SocksaddrFrom(dst.Addr(), dst.Port())
				t.NewConnectionEx(ctx, conn, source, destination, nil)
			}, true
		})
	}
	t.server.ExportLocalBackend().ExportEngine().(wgengine.ExportedUserspaceEngine).SetOnReconfigListener(t.onReconfig)

	ipStack := t.server.ExportNetstack().ExportIPStack()
	gErr := ipStack.SetSpoofing(tun.DefaultNIC, true)
	if gErr != nil {
		return gonet.TranslateNetstackError(gErr)
	}
	gErr = ipStack.SetPromiscuousMode(tun.DefaultNIC, true)
	if gErr != nil {
		return gonet.TranslateNetstackError(gErr)
	}
	icmpForwarder := tun.NewICMPForwarder(t.ctx, ipStack, t, t.udpTimeout)
	ipStack.SetTransportProtocolHandler(icmp.ProtocolNumber4, icmpForwarder.HandlePacket)
	ipStack.SetTransportProtocolHandler(icmp.ProtocolNumber6, icmpForwarder.HandlePacket)
	t.stack = ipStack
	t.icmpForwarder = icmpForwarder
	t.registerNetstackHandlers()

	localBackend := t.server.ExportLocalBackend()
	perfs := &ipn.MaskedPrefs{
		Prefs: ipn.Prefs{
			RouteAll: t.acceptRoutes,
		},
		RouteAllSet:        true,
		ExitNodeIPSet:      true,
		AdvertiseRoutesSet: true,
	}
	if len(t.advertiseRoutes) > 0 {
		perfs.AdvertiseRoutes = t.advertiseRoutes
	}
	if t.advertiseExitNode {
		perfs.AdvertiseRoutes = append(perfs.AdvertiseRoutes, tsaddr.ExitRoutes()...)
	}
	if t.relayServerPort != nil {
		perfs.RelayServerPort = t.relayServerPort
		perfs.RelayServerPortSet = true
	}
	if len(t.relayServerStaticEndpoints) > 0 {
		perfs.RelayServerStaticEndpoints = t.relayServerStaticEndpoints
		perfs.RelayServerStaticEndpointsSet = true
	}
	_, err = localBackend.EditPrefs(perfs)
	if err != nil {
		return E.Cause(err, "update prefs")
	}
	t.filter = localBackend.ExportFilter()
	go t.watchState()
	return nil
}

func (t *Endpoint) watchState() {
	localBackend := t.server.ExportLocalBackend()
	localBackend.WatchNotifications(t.ctx, ipn.NotifyInitialState, nil, func(roNotify *ipn.Notify) (keepGoing bool) {
		if roNotify.State != nil && *roNotify.State != ipn.NeedsLogin && *roNotify.State != ipn.NoState {
			return false
		}
		authURL := localBackend.StatusWithoutPeers().AuthURL
		if authURL != "" {
			t.logger.Info("Waiting for authentication: ", authURL)
			if t.platformInterface != nil {
				err := t.platformInterface.SendNotification(&adapter.Notification{
					Identifier: "tailscale-authentication",
					TypeName:   "Tailscale Authentication Notifications",
					TypeID:     10,
					Title:      "Tailscale Authentication",
					Body:       F.ToString("Tailscale outbound[", t.Tag(), "] is waiting for authentication."),
					OpenURL:    authURL,
				})
				if err != nil {
					t.logger.Error("send authentication notification: ", err)
				}
			}
			return false
		}
		return true
	})
	if t.exitNode != "" {
		localBackend.WatchNotifications(t.ctx, ipn.NotifyInitialState, nil, func(roNotify *ipn.Notify) (keepGoing bool) {
			if roNotify.State == nil || *roNotify.State != ipn.Running {
				return true
			}
			status, err := common.Must1(t.server.LocalClient()).Status(t.ctx)
			if err != nil {
				t.logger.Error("set exit node: ", err)
				return
			}
			perfs := &ipn.MaskedPrefs{
				Prefs: ipn.Prefs{
					ExitNodeAllowLANAccess: t.exitNodeAllowLANAccess,
				},
				ExitNodeIPSet:             true,
				ExitNodeAllowLANAccessSet: true,
			}
			err = perfs.SetExitNodeIP(t.exitNode, status)
			if err != nil {
				t.logger.Error("set exit node: ", err)
				return true
			}
			_, err = localBackend.EditPrefs(perfs)
			if err != nil {
				t.logger.Error("set exit node: ", err)
				return true
			}
			return false
		})
	}
}

func (t *Endpoint) Close() error {
	netmon.RegisterInterfaceGetter(nil)
	if runtime.GOOS == "android" {
		setAndroidProtectFunc(nil)
	}
	if t.fallbackTCPCloser != nil {
		t.fallbackTCPCloser()
		t.fallbackTCPCloser = nil
	}
	return common.Close(common.PtrOrNil(t.server))
}

func (t *Endpoint) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch network {
	case N.NetworkTCP:
		t.logger.InfoContext(ctx, "outbound connection to ", destination)
	case N.NetworkUDP:
		t.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	}
	if destination.IsFqdn() {
		destinationAddresses, err := t.dnsRouter.Lookup(ctx, destination.Fqdn, adapter.DNSQueryOptions{})
		if err != nil {
			return nil, err
		}
		return N.DialSerial(ctx, t, network, destination, destinationAddresses)
	}
	addr4, addr6 := t.server.TailscaleIPs()
	remoteAddr := tcpip.FullAddress{
		NIC:  1,
		Port: destination.Port,
		Addr: addressFromAddr(destination.Addr),
	}
	var localAddr tcpip.FullAddress
	var networkProtocol tcpip.NetworkProtocolNumber
	if destination.IsIPv4() {
		if !addr4.IsValid() {
			return nil, E.New("missing Tailscale IPv4 address")
		}
		networkProtocol = header.IPv4ProtocolNumber
		localAddr = tcpip.FullAddress{
			NIC:  1,
			Addr: addressFromAddr(addr4),
		}
	} else {
		if !addr6.IsValid() {
			return nil, E.New("missing Tailscale IPv6 address")
		}
		networkProtocol = header.IPv6ProtocolNumber
		localAddr = tcpip.FullAddress{
			NIC:  1,
			Addr: addressFromAddr(addr6),
		}
	}
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		tcpConn, err := gonet.DialTCPWithBind(ctx, t.stack, localAddr, remoteAddr, networkProtocol)
		if err != nil {
			return nil, err
		}
		return tcpConn, nil
	case N.NetworkUDP:
		udpConn, err := gonet.DialUDP(t.stack, &localAddr, &remoteAddr, networkProtocol)
		if err != nil {
			return nil, err
		}
		return udpConn, nil
	default:
		return nil, E.Extend(N.ErrUnknownNetwork, network)
	}
}

func (t *Endpoint) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	t.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	if destination.IsFqdn() {
		destinationAddresses, err := t.dnsRouter.Lookup(ctx, destination.Fqdn, adapter.DNSQueryOptions{})
		if err != nil {
			return nil, err
		}
		packetConn, _, err := N.ListenSerial(ctx, t, destination, destinationAddresses)
		if err != nil {
			return nil, err
		}
		return packetConn, err
	}
	addr4, addr6 := t.server.TailscaleIPs()
	bind := tcpip.FullAddress{
		NIC: 1,
	}
	var networkProtocol tcpip.NetworkProtocolNumber
	if destination.IsIPv4() {
		if !addr4.IsValid() {
			return nil, E.New("missing Tailscale IPv4 address")
		}
		networkProtocol = header.IPv4ProtocolNumber
		bind.Addr = addressFromAddr(addr4)
	} else {
		if !addr6.IsValid() {
			return nil, E.New("missing Tailscale IPv6 address")
		}
		networkProtocol = header.IPv6ProtocolNumber
		bind.Addr = addressFromAddr(addr6)
	}
	udpConn, err := gonet.DialUDP(t.stack, &bind, nil, networkProtocol)
	if err != nil {
		return nil, err
	}
	return udpConn, nil
}

func (t *Endpoint) PrepareConnection(network string, source M.Socksaddr, destination M.Socksaddr, routeContext tun.DirectRouteContext, timeout time.Duration) (tun.DirectRouteDestination, error) {
	tsFilter := t.filter.Load()
	if tsFilter != nil {
		var ipProto ipproto.Proto
		switch N.NetworkName(network) {
		case N.NetworkTCP:
			ipProto = ipproto.TCP
		case N.NetworkUDP:
			ipProto = ipproto.UDP
		case N.NetworkICMP:
			if !destination.IsIPv6() {
				ipProto = ipproto.ICMPv4
			} else {
				ipProto = ipproto.ICMPv6
			}
		}
		response := tsFilter.Check(source.Addr, destination.Addr, destination.Port, ipProto)
		switch response {
		case filter.Drop:
			return nil, syscall.ECONNREFUSED
		case filter.DropSilently:
			return nil, tun.ErrDrop
		}
	}
	var ipVersion uint8
	if !destination.IsIPv6() {
		ipVersion = 4
	} else {
		ipVersion = 6
	}
	routeDestination, err := t.router.PreMatch(adapter.InboundContext{
		Inbound:     t.Tag(),
		InboundType: t.Type(),
		IPVersion:   ipVersion,
		Network:     network,
		Source:      source,
		Destination: destination,
	}, routeContext, timeout, false)
	if err != nil {
		switch {
		case rule.IsBypassed(err):
			err = nil
		case rule.IsRejected(err):
			t.logger.Trace("reject ", network, " connection from ", source.AddrString(), " to ", destination.AddrString())
		default:
			if network == N.NetworkICMP {
				t.logger.Warn(E.Cause(err, "link ", network, " connection from ", source.AddrString(), " to ", destination.AddrString()))
			}
		}
	}
	return routeDestination, err
}

func (t *Endpoint) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	var metadata adapter.InboundContext
	metadata.Inbound = t.Tag()
	metadata.InboundType = t.Type()
	metadata.Source = source
	addr4, addr6 := t.server.TailscaleIPs()
	switch destination.Addr {
	case addr4:
		destination.Addr = netip.AddrFrom4([4]uint8{127, 0, 0, 1})
	case addr6:
		destination.Addr = netip.IPv6Loopback()
	}
	metadata.Destination = destination
	t.logger.InfoContext(ctx, "inbound connection from ", source)
	t.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
	t.router.RouteConnectionEx(ctx, conn, metadata, onClose)
}

func (t *Endpoint) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	var metadata adapter.InboundContext
	metadata.Inbound = t.Tag()
	metadata.InboundType = t.Type()
	metadata.Source = source
	addr4, addr6 := t.server.TailscaleIPs()
	switch destination.Addr {
	case addr4:
		metadata.OriginDestination = destination
		destination.Addr = netip.AddrFrom4([4]uint8{127, 0, 0, 1})
		conn = bufio.NewNATPacketConn(bufio.NewNetPacketConn(conn), metadata.OriginDestination, destination)
	case addr6:
		metadata.OriginDestination = destination
		destination.Addr = netip.IPv6Loopback()
		conn = bufio.NewNATPacketConn(bufio.NewNetPacketConn(conn), metadata.OriginDestination, destination)
	}
	metadata.Destination = destination
	t.logger.InfoContext(ctx, "inbound packet connection from ", source)
	t.logger.InfoContext(ctx, "inbound packet connection to ", metadata.Destination)
	t.router.RoutePacketConnectionEx(ctx, conn, metadata, onClose)
}

func (t *Endpoint) NewDirectRouteConnection(metadata adapter.InboundContext, routeContext tun.DirectRouteContext, timeout time.Duration) (tun.DirectRouteDestination, error) {
	inet4Address, inet6Address := t.server.TailscaleIPs()
	if metadata.Destination.Addr.Is4() && !inet4Address.IsValid() || metadata.Destination.Addr.Is6() && !inet6Address.IsValid() {
		return nil, E.New("Tailscale is not ready yet")
	}
	ctx := log.ContextWithNewID(t.ctx)
	destination, err := ping.ConnectGVisor(
		ctx, t.logger,
		metadata.Source.Addr, metadata.Destination.Addr,
		routeContext,
		t.stack,
		inet4Address, inet6Address,
		timeout,
	)
	if err != nil {
		return nil, err
	}
	t.logger.InfoContext(ctx, "linked ", metadata.Network, " connection from ", metadata.Source.AddrString(), " to ", metadata.Destination.AddrString())
	return destination, nil
}

func (t *Endpoint) PreferredDomain(domain string) bool {
	routeDomains := t.routeDomains.Load()
	if routeDomains == nil {
		return false
	}
	return routeDomains[strings.ToLower(domain)]
}

func (t *Endpoint) PreferredAddress(address netip.Addr) bool {
	routePrefixes := t.routePrefixes.Load()
	if routePrefixes == nil {
		return false
	}
	return routePrefixes.Contains(address)
}

func (t *Endpoint) Server() *tsnet.Server {
	return t.server
}

func (t *Endpoint) onReconfig(cfg *wgcfg.Config, routerCfg *router.Config, dnsCfg *tsDNS.Config) {
	if cfg == nil || dnsCfg == nil {
		return
	}
	if (t.cfg != nil && reflect.DeepEqual(t.cfg, cfg)) && (t.dnsCfg != nil && reflect.DeepEqual(t.dnsCfg, dnsCfg)) {
		return
	}
	var inet4Address, inet6Address netip.Addr
	for _, address := range cfg.Addresses {
		if address.Addr().Is4() {
			inet4Address = address.Addr()
		} else if address.Addr().Is6() {
			inet6Address = address.Addr()
		}
	}
	t.icmpForwarder.SetLocalAddresses(inet4Address, inet6Address)
	t.cfg = cfg
	t.dnsCfg = dnsCfg

	routeDomains := make(map[string]bool)
	for fqdn := range dnsCfg.Routes {
		routeDomains[fqdn.WithoutTrailingDot()] = true
	}
	for _, fqdn := range dnsCfg.SearchDomains {
		routeDomains[fqdn.WithoutTrailingDot()] = true
	}
	t.routeDomains.Store(routeDomains)

	var builder netipx.IPSetBuilder
	for _, peer := range cfg.Peers {
		for _, allowedIP := range peer.AllowedIPs {
			builder.AddPrefix(allowedIP)
		}
	}
	t.routePrefixes.Store(common.Must1(builder.IPSet()))

	if t.onReconfigHook != nil {
		t.onReconfigHook(cfg, routerCfg, dnsCfg)
	}
}

func addressFromAddr(destination netip.Addr) tcpip.Address {
	if destination.Is6() {
		return tcpip.AddrFrom16(destination.As16())
	} else {
		return tcpip.AddrFrom4(destination.As4())
	}
}

type endpointDialer struct {
	N.Dialer
	logger logger.ContextLogger
}

func (d *endpointDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		d.logger.InfoContext(ctx, "output connection to ", destination)
	case N.NetworkUDP:
		d.logger.InfoContext(ctx, "output packet connection to ", destination)
	}
	return d.Dialer.DialContext(ctx, network, destination)
}

func (d *endpointDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	d.logger.InfoContext(ctx, "output packet connection")
	return d.Dialer.ListenPacket(ctx, destination)
}

type dnsConfigurtor struct {
	baseConfig tsDNS.OSConfig
}

func (c *dnsConfigurtor) SetDNS(cfg tsDNS.OSConfig) error {
	c.baseConfig = cfg
	return nil
}

func (c *dnsConfigurtor) SupportsSplitDNS() bool {
	return true
}

func (c *dnsConfigurtor) GetBaseConfig() (tsDNS.OSConfig, error) {
	return c.baseConfig, nil
}

func (c *dnsConfigurtor) Close() error {
	return nil
}
