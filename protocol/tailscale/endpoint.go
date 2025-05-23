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
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/sagernet/gvisor/pkg/tcpip"
	"github.com/sagernet/gvisor/pkg/tcpip/adapters/gonet"
	"github.com/sagernet/gvisor/pkg/tcpip/header"
	"github.com/sagernet/gvisor/pkg/tcpip/stack"
	"github.com/sagernet/gvisor/pkg/tcpip/transport/tcp"
	"github.com/sagernet/gvisor/pkg/tcpip/transport/udp"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/endpoint"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/libbox/platform"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/filemanager"
	"github.com/sagernet/tailscale/ipn"
	tsDNS "github.com/sagernet/tailscale/net/dns"
	"github.com/sagernet/tailscale/net/netmon"
	"github.com/sagernet/tailscale/net/tsaddr"
	"github.com/sagernet/tailscale/tsnet"
	"github.com/sagernet/tailscale/types/ipproto"
	"github.com/sagernet/tailscale/version"
	"github.com/sagernet/tailscale/wgengine"
	"github.com/sagernet/tailscale/wgengine/filter"
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
	platformInterface platform.Interface
	server            *tsnet.Server
	stack             *stack.Stack
	filter            *atomic.Pointer[filter.Filter]
	onReconfig        wgengine.ReconfigListener

	acceptRoutes           bool
	exitNode               string
	exitNodeAllowLANAccess bool
	advertiseRoutes        []netip.Prefix
	advertiseExitNode      bool

	udpTimeout time.Duration
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
				},
			},
		},
	}
	return &Endpoint{
		Adapter:                endpoint.NewAdapter(C.TypeTailscale, tag, []string{N.NetworkTCP, N.NetworkUDP}, nil),
		ctx:                    ctx,
		router:                 router,
		logger:                 logger,
		dnsRouter:              dnsRouter,
		network:                service.FromContext[adapter.NetworkManager](ctx),
		platformInterface:      service.FromContext[platform.Interface](ctx),
		server:                 server,
		acceptRoutes:           options.AcceptRoutes,
		exitNode:               options.ExitNode,
		exitNodeAllowLANAccess: options.ExitNodeAllowLANAccess,
		advertiseRoutes:        options.AdvertiseRoutes,
		advertiseExitNode:      options.AdvertiseExitNode,
		udpTimeout:             udpTimeout,
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
	err := t.server.Start()
	if err != nil {
		return err
	}
	if t.onReconfig != nil {
		t.server.ExportLocalBackend().ExportEngine().(wgengine.ExportedUserspaceEngine).SetOnReconfigListener(t.onReconfig)
	}

	ipStack := t.server.ExportNetstack().ExportIPStack()
	gErr := ipStack.SetSpoofing(tun.DefaultNIC, true)
	if gErr != nil {
		return gonet.TranslateNetstackError(gErr)
	}
	gErr = ipStack.SetPromiscuousMode(tun.DefaultNIC, true)
	if gErr != nil {
		return gonet.TranslateNetstackError(gErr)
	}
	ipStack.SetTransportProtocolHandler(tcp.ProtocolNumber, tun.NewTCPForwarder(t.ctx, ipStack, t).HandlePacket)
	udpForwarder := tun.NewUDPForwarder(t.ctx, ipStack, t, t.udpTimeout)
	ipStack.SetTransportProtocolHandler(udp.ProtocolNumber, udpForwarder.HandlePacket)
	t.stack = ipStack

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
				err := t.platformInterface.SendNotification(&platform.Notification{
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
	addr := tcpip.FullAddress{
		NIC:  1,
		Port: destination.Port,
		Addr: addressFromAddr(destination.Addr),
	}
	var networkProtocol tcpip.NetworkProtocolNumber
	if destination.IsIPv4() {
		networkProtocol = header.IPv4ProtocolNumber
	} else {
		networkProtocol = header.IPv6ProtocolNumber
	}
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		tcpConn, err := gonet.DialContextTCP(ctx, t.stack, addr, networkProtocol)
		if err != nil {
			return nil, err
		}
		return tcpConn, nil
	case N.NetworkUDP:
		udpConn, err := gonet.DialUDP(t.stack, nil, &addr, networkProtocol)
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

func (t *Endpoint) PrepareConnection(network string, source M.Socksaddr, destination M.Socksaddr) error {
	tsFilter := t.filter.Load()
	if tsFilter != nil {
		var ipProto ipproto.Proto
		switch N.NetworkName(network) {
		case N.NetworkTCP:
			ipProto = ipproto.TCP
		case N.NetworkUDP:
			ipProto = ipproto.UDP
		}
		response := tsFilter.Check(source.Addr, destination.Addr, destination.Port, ipProto)
		switch response {
		case filter.Drop:
			return syscall.ECONNRESET
		case filter.DropSilently:
			return tun.ErrDrop
		}
	}
	return t.router.PreMatch(adapter.InboundContext{
		Inbound:     t.Tag(),
		InboundType: t.Type(),
		Network:     network,
		Source:      source,
		Destination: destination,
	})
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
	metadata.Destination = destination
	addr4, addr6 := t.server.TailscaleIPs()
	switch destination.Addr {
	case addr4:
		metadata.OriginDestination = destination
		destination.Addr = netip.AddrFrom4([4]uint8{127, 0, 0, 1})
		conn = bufio.NewNATPacketConn(bufio.NewNetPacketConn(conn), metadata.OriginDestination, metadata.Destination)
	case addr6:
		metadata.OriginDestination = destination
		destination.Addr = netip.IPv6Loopback()
		conn = bufio.NewNATPacketConn(bufio.NewNetPacketConn(conn), metadata.OriginDestination, metadata.Destination)
	}
	t.logger.InfoContext(ctx, "inbound packet connection from ", source)
	t.logger.InfoContext(ctx, "inbound packet connection to ", destination)
	t.router.RoutePacketConnectionEx(ctx, conn, metadata, onClose)
}

func (t *Endpoint) Server() *tsnet.Server {
	return t.server
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
