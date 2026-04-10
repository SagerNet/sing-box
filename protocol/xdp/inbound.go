//go:build linux && with_gvisor

package xdp

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/sagernet/gvisor/pkg/tcpip"
	"github.com/sagernet/gvisor/pkg/tcpip/network/ipv4"
	"github.com/sagernet/gvisor/pkg/tcpip/network/ipv6"
	"github.com/sagernet/gvisor/pkg/tcpip/stack"
	"github.com/sagernet/gvisor/pkg/tcpip/transport/icmp"
	"github.com/sagernet/gvisor/pkg/tcpip/transport/tcp"
	"github.com/sagernet/gvisor/pkg/tcpip/transport/udp"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/common/taskmonitor"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	tun "github.com/sagernet/sing-tun"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service"
)

func RegisterInbound(registry *inbound.Registry) {
	inbound.Register[option.XDPInboundOptions](registry, C.TypeXDP, NewInbound)
}

type Inbound struct {
	tag        string
	ctx        context.Context
	router     adapter.Router
	logger     log.ContextLogger
	options    option.XDPInboundOptions
	udpTimeout time.Duration

	// network change monitoring — keeps local IP hint map in sync
	networkManager  adapter.NetworkManager
	networkCallback *list.Element[tun.NetworkUpdateCallback]
	localAddresses  []netip.Addr // host IPs in the sk_lookup hint map

	// runtime state
	xdpProg  *XDPProgram
	xsks     []*XSKSocket
	endpoint *xdpEndpoint
	ipStack  *stack.Stack
	ifIndex  int
}

func NewInbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.XDPInboundOptions) (adapter.Inbound, error) {
	if options.Interface == "" {
		return nil, E.New("missing required field: interface")
	}

	var udpTimeout time.Duration
	if options.UDPTimeout != 0 {
		udpTimeout = time.Duration(options.UDPTimeout)
	} else {
		udpTimeout = C.UDPTimeout
	}

	if options.MTU == 0 {
		options.MTU = 1500
	}
	if options.FrameSize == 0 {
		options.FrameSize = defaultFrameSize
	}
	if options.FrameCount == 0 {
		options.FrameCount = defaultFrameCount
	}

	return &Inbound{
		tag:            tag,
		ctx:            ctx,
		router:         router,
		logger:         logger,
		options:        options,
		udpTimeout:     udpTimeout,
		networkManager: service.FromContext[adapter.NetworkManager](ctx),
	}, nil
}

func (i *Inbound) Type() string {
	return C.TypeXDP
}

func (i *Inbound) Tag() string {
	return i.tag
}

func (i *Inbound) Start(stage adapter.StartStage) error {
	switch stage {
	case adapter.StartStateStart:
		return i.startXDP()
	case adapter.StartStatePostStart:
		i.registerNetworkCallback()
		i.logger.Info("XDP data plane started on ", i.options.Interface, " (ifindex=", i.ifIndex, ")")
	}
	return nil
}

func (i *Inbound) startXDP() error {
	monitor := taskmonitor.New(i.logger, C.StartTimeout)

	if err := checkKernelVersion(); err != nil {
		return E.Cause(err, "kernel compatibility check failed")
	}

	monitor.Start("resolve interface")
	iface, err := net.InterfaceByName(i.options.Interface)
	if err != nil {
		return E.Cause(err, "resolve interface ", i.options.Interface)
	}
	i.ifIndex = iface.Index
	monitor.Finish()

	monitor.Start("load XDP program")
	i.xdpProg, err = LoadXDPProgram(xdpProgData, i.ifIndex)
	if err != nil {
		return E.Cause(err, "load XDP program")
	}
	i.logger.Info("loaded embedded XDP program")
	monitor.Finish()

	monitor.Start("populate BPF maps")
	if err := i.populateMaps(); err != nil {
		return E.Cause(err, "populate BPF maps")
	}
	monitor.Finish()

	monitor.Start("create AF_XDP sockets")
	numQueues := detectRXQueueCount(i.options.Interface)
	i.logger.Info("detected ", numQueues, " RX queue(s) on ", i.options.Interface)

	perQueueFrames := i.options.FrameCount / uint32(numQueues)
	if perQueueFrames < 256 {
		perQueueFrames = 256
	}

	var activeXSKs []*XSKSocket
	for q := 0; q < numQueues; q++ {
		xsk, xskErr := NewXSKSocket(i.ifIndex, q, i.options.FrameSize, perQueueFrames)
		if xskErr != nil {
			i.logger.Warn("skip queue ", q, ": ", xskErr)
			continue
		}
		if regErr := i.xdpProg.RegisterXSK(q, xsk.FD()); regErr != nil {
			xsk.Close()
			i.logger.Warn("register XSK for queue ", q, ": ", regErr)
			continue
		}
		activeXSKs = append(activeXSKs, xsk)
		i.logger.Info("created AF_XDP socket on queue ", q)
	}
	if len(activeXSKs) == 0 {
		return E.New("failed to create any AF_XDP socket")
	}
	i.xsks = activeXSKs
	monitor.Finish()

	monitor.Start("create gVisor netstack")
	i.endpoint = newXDPEndpoint(activeXSKs, i.options.MTU, iface.HardwareAddr)

	ipStack, err := tun.NewGVisorStackWithOptions(i.endpoint, stack.NICOptions{}, true)
	if err != nil {
		return E.Cause(err, "create gVisor stack")
	}
	i.ipStack = ipStack

	// If not configured, auto-detect from the network interface
	addresses := i.options.Address
	if len(addresses) == 0 {
		addrs, addrErr := iface.Addrs()
		if addrErr != nil {
			return E.Cause(addrErr, "get interface addresses")
		}
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok {
				if prefix, pOk := netip.AddrFromSlice(ipNet.IP); pOk {
					ones, _ := ipNet.Mask.Size()
					addresses = append(addresses, netip.PrefixFrom(prefix.Unmap(), ones))
				}
			}
		}
		if len(addresses) == 0 {
			return E.New("no IP addresses found on interface ", i.options.Interface)
		}
		i.logger.Info("auto-detected addresses: ", fmt.Sprint(addresses))
	}

	var (
		inet4Address netip.Addr
		inet6Address netip.Addr
	)
	for _, prefix := range addresses {
		addr := tun.AddressFromAddr(prefix.Addr())
		protoAddr := tcpip.ProtocolAddress{
			AddressWithPrefix: tcpip.AddressWithPrefix{
				Address:   addr,
				PrefixLen: prefix.Bits(),
			},
		}
		if prefix.Addr().Is4() {
			inet4Address = prefix.Addr()
			protoAddr.Protocol = ipv4.ProtocolNumber
		} else {
			inet6Address = prefix.Addr()
			protoAddr.Protocol = ipv6.ProtocolNumber
		}
		gErr := ipStack.AddProtocolAddress(tun.DefaultNIC, protoAddr, stack.AddressProperties{})
		if gErr != nil {
			return E.New("add address ", protoAddr.AddressWithPrefix, ": ", gErr.String())
		}
	}

	// Set up TCP/UDP/ICMP forwarders → route to sing-box router
	ipStack.SetTransportProtocolHandler(tcp.ProtocolNumber,
		tun.NewTCPForwarder(i.ctx, ipStack, i).HandlePacket)
	ipStack.SetTransportProtocolHandler(udp.ProtocolNumber,
		tun.NewUDPForwarder(i.ctx, ipStack, i, i.udpTimeout).HandlePacket)
	icmpForwarder := tun.NewICMPForwarder(i.ctx, ipStack, i, i.udpTimeout)
	icmpForwarder.SetLocalAddresses(inet4Address, inet6Address)
	ipStack.SetTransportProtocolHandler(icmp.ProtocolNumber4, icmpForwarder.HandlePacket)
	ipStack.SetTransportProtocolHandler(icmp.ProtocolNumber6, icmpForwarder.HandlePacket)

	monitor.Finish()
	i.logger.Info("gVisor netstack created with addresses ", fmt.Sprint(addresses))

	return nil
}

func (i *Inbound) populateMaps() error {
	// route_address: destination CIDRs to capture
	if len(i.options.RouteAddress) > 0 {
		for _, prefix := range i.options.RouteAddress {
			if err := i.xdpProg.AddRoutePrefix(prefix); err != nil {
				return E.Cause(err, "add route address ", prefix)
			}
		}
		i.logger.Info("route addresses: ", fmt.Sprint(i.options.RouteAddress))
	} else {
		// Default: capture all traffic
		if err := i.xdpProg.AddRoutePrefix(netip.MustParsePrefix("0.0.0.0/0")); err != nil {
			return E.Cause(err, "add default IPv4 route")
		}
		if err := i.xdpProg.AddRoutePrefix(netip.MustParsePrefix("::/0")); err != nil {
			return E.Cause(err, "add default IPv6 route")
		}
		i.logger.Info("route address: default (capture all)")
	}

	// route_exclude_address: destination CIDRs to exclude
	for _, prefix := range i.options.RouteExcludeAddress {
		if err := i.xdpProg.AddRouteExcludePrefix(prefix); err != nil {
			return E.Cause(err, "add route exclude address ", prefix)
		}
	}
	if len(i.options.RouteExcludeAddress) > 0 {
		i.logger.Info("route exclude addresses: ", fmt.Sprint(i.options.RouteExcludeAddress))
	}

	// Populate local IP hint map for bpf_sk_lookup gating
	localAddrs, localErr := collectLocalAddresses()
	if localErr != nil {
		i.logger.Warn("failed to collect local addresses for hint map: ", localErr)
	} else {
		for _, addr := range localAddrs {
			if err := i.xdpProg.AddLocalIPHint(addr); err != nil {
				return E.Cause(err, "add local IP hint ", addr)
			}
		}
		i.localAddresses = localAddrs
		if len(localAddrs) > 0 {
			i.logger.Info("local IP hints (sk_lookup gate): ", fmt.Sprint(localAddrs))
		}
	}

	return nil
}

func containsAddr(addrs []netip.Addr, addr netip.Addr) bool {
	for _, a := range addrs {
		if a == addr {
			return true
		}
	}
	return false
}

// addrsEqual reports whether two address slices contain the same addresses (order-independent).
func addrsEqual(a, b []netip.Addr) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[netip.Addr]struct{}, len(a))
	for _, addr := range a {
		set[addr] = struct{}{}
	}
	for _, addr := range b {
		if _, ok := set[addr]; !ok {
			return false
		}
	}
	return true
}

// collectLocalAddresses returns all loopback and globally unicast IP addresses
// present on the host's network interfaces.
func collectLocalAddresses() ([]netip.Addr, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	seen := make(map[netip.Addr]struct{})
	var result []netip.Addr
	for _, iface := range ifaces {
		addrs, aErr := iface.Addrs()
		if aErr != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip, pOk := netip.AddrFromSlice(ipNet.IP)
			if !pOk {
				continue
			}
			ip = ip.Unmap()
			// Include loopback and globally unicast only.
			// Link-local (fe80::/10) and other special-use addresses are skipped.
			if !ip.IsLoopback() && !ip.IsGlobalUnicast() {
				continue
			}
			if _, dup := seen[ip]; !dup {
				seen[ip] = struct{}{}
				result = append(result, ip)
			}
		}
	}
	return result, nil
}

// detectRXQueueCount reads /sys/class/net/<iface>/queues/ to count rx-* entries.
// Returns 1 on failure (safe fallback for single-queue NICs).
func detectRXQueueCount(ifaceName string) int {
	entries, err := os.ReadDir(fmt.Sprintf("/sys/class/net/%s/queues", ifaceName))
	if err != nil {
		return 1
	}
	count := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "rx-") {
			count++
		}
	}
	if count == 0 {
		return 1
	}
	return count
}

func (i *Inbound) registerNetworkCallback() {
	if i.networkManager == nil {
		return
	}
	i.networkCallback = i.networkManager.NetworkMonitor().RegisterCallback(i.updateLocalAddresses)
	i.logger.Debug("registered network change callback for local IP hint map")
}

// updateLocalAddresses re-scans local IPs on network change
// and updates the BPF hint map accordingly.
func (i *Inbound) updateLocalAddresses() {
	if i.xdpProg == nil {
		return
	}

	newAddrs, err := collectLocalAddresses()
	if err != nil {
		i.logger.Warn("failed to collect local addresses for hint map update: ", err)
		return
	}

	if addrsEqual(newAddrs, i.localAddresses) {
		return
	}

	for _, old := range i.localAddresses {
		if !containsAddr(newAddrs, old) {
			_ = i.xdpProg.DeleteLocalIPHint(old)
		}
	}

	for _, addr := range newAddrs {
		if !containsAddr(i.localAddresses, addr) {
			_ = i.xdpProg.AddLocalIPHint(addr)
		}
	}

	i.localAddresses = newAddrs
	i.logger.Info("updated local IP hints: ", fmt.Sprint(newAddrs))
}

func (i *Inbound) Close() error {
	if i.networkCallback != nil && i.networkManager != nil {
		i.networkManager.NetworkMonitor().UnregisterCallback(i.networkCallback)
	}
	if i.ipStack != nil {
		i.ipStack.Close()
	}
	if i.endpoint != nil {
		i.endpoint.Close()
	}
	for _, xsk := range i.xsks {
		if xsk != nil {
			xsk.Close()
		}
	}
	if i.xdpProg != nil {
		i.xdpProg.Close()
	}
	return nil
}

func (i *Inbound) PrepareConnection(network string, source M.Socksaddr, destination M.Socksaddr, _ tun.DirectRouteContext, _ time.Duration) (tun.DirectRouteDestination, error) {
	return nil, nil
}

func (i *Inbound) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	ctx = log.ContextWithNewID(ctx)
	var metadata adapter.InboundContext
	metadata.Inbound = i.tag
	metadata.InboundType = C.TypeXDP
	metadata.Source = source
	metadata.Destination = destination

	i.logger.InfoContext(ctx, "inbound connection from ", metadata.Source)
	i.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
	i.router.RouteConnectionEx(ctx, conn, metadata, onClose)
}

func (i *Inbound) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	ctx = log.ContextWithNewID(ctx)
	var metadata adapter.InboundContext
	metadata.Inbound = i.tag
	metadata.InboundType = C.TypeXDP
	metadata.Source = source
	metadata.Destination = destination

	i.logger.InfoContext(ctx, "inbound packet connection from ", metadata.Source)
	i.logger.InfoContext(ctx, "inbound packet connection to ", metadata.Destination)
	i.router.RoutePacketConnectionEx(ctx, conn, metadata, onClose)
}
