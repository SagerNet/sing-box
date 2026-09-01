package tun

import (
	"context"
	"net"
	"net/netip"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/common/taskmonitor"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing-tun/gtcpip/header"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/ranges"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service"

	"go4.org/netipx"
)

func RegisterInbound(registry *inbound.Registry) {
	inbound.Register[option.TunInboundOptions](registry, C.TypeTun, NewInbound)
}

type Inbound struct {
	tag                         string
	ctx                         context.Context
	router                      adapter.Router
	networkManager              adapter.NetworkManager
	logger                      log.ContextLogger
	tunOptions                  tun.Options
	udpTimeout                  time.Duration
	udpMapping                  tun.NATMapping
	udpFiltering                tun.NATFiltering
	udpNATMax                   uint32
	dnsHijackAddress            []netip.Addr
	dnsHijackByPort             bool
	stack                       string
	tunIf                       tun.Tun
	tunStack                    tun.Stack
	platformInterface           adapter.PlatformInterface
	platformOptions             option.TunPlatformOptions
	autoRedirect                tun.AutoRedirect
	routeRuleSet                []adapter.RuleSet
	routeRuleSetCallback        []*list.Element[adapter.RuleSetUpdateCallback]
	routeExcludeRuleSet         []adapter.RuleSet
	routeExcludeRuleSetCallback []*list.Element[adapter.RuleSetUpdateCallback]
	routeAddressSetAccess       sync.RWMutex
	routeAddressSet             []*netipx.IPSet
	routeExcludeAddressSet      []*netipx.IPSet
}

func NewInbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.TunInboundOptions) (adapter.Inbound, error) {
	//nolint:staticcheck
	if len(options.Inet4Address) > 0 || len(options.Inet6Address) > 0 ||
		len(options.Inet4RouteAddress) > 0 || len(options.Inet6RouteAddress) > 0 ||
		len(options.Inet4RouteExcludeAddress) > 0 || len(options.Inet6RouteExcludeAddress) > 0 {
		return nil, E.New("legacy tun address fields are deprecated in sing-box 1.10.0 and removed in sing-box 1.12.0")
	}
	//nolint:staticcheck
	if options.GSO {
		return nil, E.New("GSO option in tun is deprecated in sing-box 1.11.0 and removed in sing-box 1.12.0")
	}
	//nolint:staticcheck
	if options.InboundOptions != (option.InboundOptions{}) {
		return nil, E.New("legacy inbound fields are deprecated in sing-box 1.11.0 and removed in sing-box 1.13.0, checkout migration: https://sing-box.sagernet.org/migration/#migrate-legacy-inbound-fields-to-rule-actions")
	}

	address := options.Address
	inet4Address := common.Filter(address, func(it netip.Prefix) bool {
		return it.Addr().Is4()
	})
	inet6Address := common.Filter(address, func(it netip.Prefix) bool {
		return it.Addr().Is6()
	})

	routeAddress := options.RouteAddress
	inet4RouteAddress := common.Filter(routeAddress, func(it netip.Prefix) bool {
		return it.Addr().Is4()
	})
	inet6RouteAddress := common.Filter(routeAddress, func(it netip.Prefix) bool {
		return it.Addr().Is6()
	})

	routeExcludeAddress := options.RouteExcludeAddress
	inet4RouteExcludeAddress := common.Filter(routeExcludeAddress, func(it netip.Prefix) bool {
		return it.Addr().Is4()
	})
	inet6RouteExcludeAddress := common.Filter(routeExcludeAddress, func(it netip.Prefix) bool {
		return it.Addr().Is6()
	})

	platformInterface := service.FromContext[adapter.PlatformInterface](ctx)
	if options.NetNs != "" && !C.IsLinux {
		return nil, E.New("`netns` is only supported on Linux")
	}
	tunMTU := options.MTU
	if tunMTU == 0 {
		if platformInterface != nil && platformInterface.UnderNetworkExtension() {
			// In Network Extension, when MTU exceeds 4064 (4096-UTUN_IF_HEADROOM_SIZE), the performance of tun will drop significantly, which may be a system bug.
			tunMTU = 4064
		} else if C.IsAndroid {
			// Some Android devices report ENOBUFS when using MTU 65535
			tunMTU = 9000
		} else {
			tunMTU = 65535
		}
	}
	var enableGSO bool
	if C.IsLinux && platformInterface == nil {
		enableGSO = (options.Stack == "gvisor" && tunMTU < 49152)
	}
	var udpTimeout time.Duration
	if options.UDPTimeout != 0 {
		udpTimeout = time.Duration(options.UDPTimeout)
	} else {
		udpTimeout = C.UDPTimeout
	}
	var err error
	includeUID := uidToRange(options.IncludeUID)
	if len(options.IncludeUIDRange) > 0 {
		includeUID, err = parseRange(includeUID, options.IncludeUIDRange)
		if err != nil {
			return nil, E.Cause(err, "parse include_uid_range")
		}
	}
	excludeUID := uidToRange(options.ExcludeUID)
	if len(options.ExcludeUIDRange) > 0 {
		excludeUID, err = parseRange(excludeUID, options.ExcludeUIDRange)
		if err != nil {
			return nil, E.Cause(err, "parse exclude_uid_range")
		}
	}

	tableIndex := options.IPRoute2TableIndex
	if tableIndex == 0 {
		tableIndex = tun.DefaultIPRoute2TableIndex
	}
	ruleIndex := options.IPRoute2RuleIndex
	if ruleIndex == 0 {
		ruleIndex = tun.DefaultIPRoute2RuleIndex
	}
	autoRedirectFallbackRuleIndex := options.AutoRedirectFallbackRuleIndex
	if autoRedirectFallbackRuleIndex == 0 {
		autoRedirectFallbackRuleIndex = tun.DefaultIPRoute2AutoRedirectFallbackRuleIndex
	}
	nfQueue := options.AutoRedirectNFQueue
	if nfQueue == 0 {
		nfQueue = tun.DefaultAutoRedirectNFQueue
	}
	var includeMACAddress []net.HardwareAddr
	for i, macString := range options.IncludeMACAddress {
		mac, macErr := net.ParseMAC(macString)
		if macErr != nil {
			return nil, E.Cause(macErr, "parse include_mac_address[", i, "]")
		}
		includeMACAddress = append(includeMACAddress, mac)
	}
	var excludeMACAddress []net.HardwareAddr
	for i, macString := range options.ExcludeMACAddress {
		mac, macErr := net.ParseMAC(macString)
		if macErr != nil {
			return nil, E.Cause(macErr, "parse exclude_mac_address[", i, "]")
		}
		excludeMACAddress = append(excludeMACAddress, mac)
	}
	networkManager := service.FromContext[adapter.NetworkManager](ctx)
	multiPendingPackets := C.IsDarwin && ((options.Stack == "gvisor" && tunMTU < 32768) || (options.Stack != "gvisor" && tunMTU <= 9000))
	inbound := &Inbound{
		tag:            tag,
		ctx:            ctx,
		router:         router,
		networkManager: networkManager,
		logger:         logger,
		tunOptions: tun.Options{
			Name:                                  options.InterfaceName,
			NetNs:                                 options.NetNs,
			MTU:                                   tunMTU,
			GSO:                                   enableGSO,
			Inet4Address:                          inet4Address,
			Inet6Address:                          inet6Address,
			DNSMode:                               options.DNSMode,
			DNSAddress:                            options.DNSAddress,
			AutoRoute:                             options.AutoRoute,
			IPRoute2TableIndex:                    tableIndex,
			IPRoute2RuleIndex:                     ruleIndex,
			IPRoute2AutoRedirectFallbackRuleIndex: autoRedirectFallbackRuleIndex,
			AutoRedirectInputMark:                 uint32(options.AutoRedirectInputMark),
			AutoRedirectOutputMark:                uint32(options.AutoRedirectOutputMark),
			AutoRedirectResetMark:                 uint32(options.AutoRedirectResetMark),
			AutoRedirectTProxyMark:                uint32(options.AutoRedirectTProxyMark),
			AutoRedirectNFQueue:                   nfQueue,
			ExcludeMPTCP:                          options.ExcludeMPTCP,
			Inet4LoopbackAddress:                  common.Filter(options.LoopbackAddress, netip.Addr.Is4),
			Inet6LoopbackAddress:                  common.Filter(options.LoopbackAddress, netip.Addr.Is6),
			StrictRoute:                           options.StrictRoute,
			IncludeInterface:                      options.IncludeInterface,
			ExcludeInterface:                      options.ExcludeInterface,
			Inet4RouteAddress:                     inet4RouteAddress,
			Inet6RouteAddress:                     inet6RouteAddress,
			Inet4RouteExcludeAddress:              inet4RouteExcludeAddress,
			Inet6RouteExcludeAddress:              inet6RouteExcludeAddress,
			IncludeUID:                            includeUID,
			ExcludeUID:                            excludeUID,
			IncludeAndroidUser:                    options.IncludeAndroidUser,
			IncludePackage:                        options.IncludePackage,
			ExcludePackage:                        options.ExcludePackage,
			IncludeMACAddress:                     includeMACAddress,
			ExcludeMACAddress:                     excludeMACAddress,
			InterfaceMonitor:                      networkManager.InterfaceMonitor(),
			Logger:                                logger,
			EXP_MultiPendingPackets:               multiPendingPackets,
		},
		udpTimeout:        udpTimeout,
		udpMapping:        tun.NATMapping(options.UDPMapping),
		udpFiltering:      tun.NATFiltering(options.UDPFiltering),
		udpNATMax:         options.UDPNATMax,
		stack:             options.Stack,
		platformInterface: platformInterface,
		platformOptions:   common.PtrValueOrDefault(options.Platform),
	}
	for _, routeAddressSet := range options.RouteAddressSet {
		ruleSet, loaded := router.RuleSet(routeAddressSet)
		if !loaded {
			return nil, E.New("parse route_address_set: rule-set not found: ", routeAddressSet)
		}
		inbound.routeRuleSet = append(inbound.routeRuleSet, ruleSet)
	}
	for _, routeExcludeAddressSet := range options.RouteExcludeAddressSet {
		ruleSet, loaded := router.RuleSet(routeExcludeAddressSet)
		if !loaded {
			return nil, E.New("parse route_exclude_address_set: rule-set not found: ", routeExcludeAddressSet)
		}
		inbound.routeExcludeRuleSet = append(inbound.routeExcludeRuleSet, ruleSet)
	}
	if options.AutoRedirect {
		if !options.AutoRoute {
			return nil, E.New("`auto_route` is required by `auto_redirect`")
		}
		inbound.tunOptions.AutoRedirectMarkMode = true
		usePlatformAutoRedirect := platformInterface != nil && platformInterface.UsePlatformAutoRedirect()
		if usePlatformAutoRedirect {
			inbound.autoRedirect, err = newPlatformAutoRedirect(inbound)
		} else {
			disableNFTables, parseErr := strconv.ParseBool(os.Getenv("DISABLE_NFTABLES"))
			inbound.autoRedirect, err = tun.NewAutoRedirect(tun.AutoRedirectOptions{
				TunOptions:      &inbound.tunOptions,
				Context:         ctx,
				Handler:         (*autoRedirectHandler)(inbound),
				Logger:          logger,
				NetworkMonitor:  networkManager.NetworkMonitor(),
				InterfaceFinder: networkManager.InterfaceFinder(),
				TableName:       "sing-box",
				DisableNFTables: parseErr == nil && disableNFTables,
			})
		}
		if err != nil {
			return nil, E.Cause(err, "initialize auto-redirect")
		}
		inbound.dnsHijackByPort = inbound.tunOptions.DNSModeOrDefault() == tun.DNSModeHijack
		if !usePlatformAutoRedirect && options.NetNs == "" {
			err = networkManager.RegisterAutoRedirectOutputMark(inbound.tunOptions.AutoRedirectOutputMark)
			if err != nil {
				return nil, err
			}
		}
	}
	return inbound, nil
}

func uidToRange(uidList badoption.Listable[uint32]) []ranges.Range[uint32] {
	return common.Map(uidList, func(uid uint32) ranges.Range[uint32] {
		return ranges.NewSingle(uid)
	})
}

func parseRange(uidRanges []ranges.Range[uint32], rangeList []string) ([]ranges.Range[uint32], error) {
	for _, uidRange := range rangeList {
		if !strings.Contains(uidRange, ":") {
			return nil, E.New("missing ':' in range: ", uidRange)
		}
		subIndex := strings.Index(uidRange, ":")
		if subIndex == 0 {
			return nil, E.New("missing range start: ", uidRange)
		} else if subIndex == len(uidRange)-1 {
			return nil, E.New("missing range end: ", uidRange)
		}
		var start, end uint64
		var err error
		start, err = strconv.ParseUint(uidRange[:subIndex], 0, 32)
		if err != nil {
			return nil, E.Cause(err, "parse range start")
		}
		end, err = strconv.ParseUint(uidRange[subIndex+1:], 0, 32)
		if err != nil {
			return nil, E.Cause(err, "parse range end")
		}
		uidRanges = append(uidRanges, ranges.New(uint32(start), uint32(end)))
	}
	return uidRanges, nil
}

func (t *Inbound) Type() string {
	return C.TypeTun
}

func (t *Inbound) Tag() string {
	return t.tag
}

func (t *Inbound) Start(stage adapter.StartStage) error {
	switch stage {
	case adapter.StartStateInitialize:
		if t.tunOptions.DNSModeOrDefault() != tun.DNSModeDisabled && len(t.tunOptions.DNSAddress) == 0 {
			inet4DNSAddress, _ := t.tunOptions.Inet4DNSAddress()
			inet6DNSAddress, _ := t.tunOptions.Inet6DNSAddress()
			t.dnsHijackAddress = append(inet4DNSAddress, inet6DNSAddress...)
		}
	case adapter.StartStateStart:
		if t.platformInterface == nil &&
			((C.IsLinux && !t.tunOptions.GSO) || (C.IsDarwin && !t.tunOptions.EXP_MultiPendingPackets)) {
			outboundManager := service.FromContext[adapter.OutboundManager](t.ctx)
			endpointManager := service.FromContext[adapter.EndpointManager](t.ctx)
			for _, outbound := range outboundManager.Outbounds() {
				if _, isFlowOutbound := outbound.(adapter.FlowOutbound); isFlowOutbound && common.Contains(outbound.Network(), N.NetworkTCP) {
					if C.IsLinux {
						t.tunOptions.GSO = true
					} else {
						t.tunOptions.EXP_MultiPendingPackets = true
					}
					break
				}
			}
			for _, endpoint := range endpointManager.Endpoints() {
				if _, isFlowOutbound := endpoint.(adapter.FlowOutbound); isFlowOutbound && common.Contains(endpoint.Network(), N.NetworkTCP) {
					if C.IsLinux {
						t.tunOptions.GSO = true
					} else {
						t.tunOptions.EXP_MultiPendingPackets = true
					}
					break
				}
			}
		}
		if C.IsAndroid && t.platformInterface == nil {
			t.tunOptions.BuildAndroidRules(t.networkManager.PackageManager())
		}
		if t.tunOptions.Name == "" {
			t.tunOptions.Name = tun.CalculateInterfaceName("")
		}
		if t.tunOptions.NetNs != "" {
			manager := service.FromContext[adapter.NetworkNamespaceManager](t.ctx)
			if manager != nil {
				t.tunOptions.NetNs = manager.ResolvePath(t.tunOptions.NetNs)
			}
		}
		var (
			routeAddressSet        []*netipx.IPSet
			routeExcludeAddressSet []*netipx.IPSet
		)
		if t.autoRedirect != nil || t.platformInterface == nil || C.IsWindows {
			for _, routeRuleSet := range t.routeRuleSet {
				ipSets := routeRuleSet.ExtractIPSet()
				if len(ipSets) == 0 {
					t.logger.Warn("route_address_set: no destination IP CIDR rules found in rule-set: ", routeRuleSet.Name())
				}
				routeRuleSet.IncRef()
				routeAddressSet = append(routeAddressSet, ipSets...)
			}
			for _, routeExcludeRuleSet := range t.routeExcludeRuleSet {
				ipSets := routeExcludeRuleSet.ExtractIPSet()
				if len(ipSets) == 0 {
					t.logger.Warn("route_exclude_address_set: no destination IP CIDR rules found in rule-set: ", routeExcludeRuleSet.Name())
				}
				routeExcludeRuleSet.IncRef()
				routeExcludeAddressSet = append(routeExcludeAddressSet, ipSets...)
			}
			if t.autoRedirect != nil {
				t.routeAddressSetAccess.Lock()
				t.routeAddressSet = routeAddressSet
				t.routeExcludeAddressSet = routeExcludeAddressSet
				t.routeAddressSetAccess.Unlock()
				for _, routeRuleSet := range t.routeRuleSet {
					t.routeRuleSetCallback = append(t.routeRuleSetCallback, routeRuleSet.RegisterCallback(t.updateRouteAddressSet))
				}
				for _, routeExcludeRuleSet := range t.routeExcludeRuleSet {
					t.routeExcludeRuleSetCallback = append(t.routeExcludeRuleSetCallback, routeExcludeRuleSet.RegisterCallback(t.updateRouteAddressSet))
				}
			}
		}
		var (
			tunInterface tun.Tun
			err          error
		)
		monitor := taskmonitor.New(t.logger, C.StartTimeout)
		tunOptions := t.tunOptions
		if t.autoRedirect == nil && !(runtime.GOOS == "android" && t.platformInterface != nil) {
			for _, ipSet := range routeAddressSet {
				for _, prefix := range ipSet.Prefixes() {
					if prefix.Addr().Is4() {
						tunOptions.Inet4RouteAddress = append(tunOptions.Inet4RouteAddress, prefix)
					} else {
						tunOptions.Inet6RouteAddress = append(tunOptions.Inet6RouteAddress, prefix)
					}
				}
			}
			for _, ipSet := range routeExcludeAddressSet {
				for _, prefix := range ipSet.Prefixes() {
					if prefix.Addr().Is4() {
						tunOptions.Inet4RouteExcludeAddress = append(tunOptions.Inet4RouteExcludeAddress, prefix)
					} else {
						tunOptions.Inet6RouteExcludeAddress = append(tunOptions.Inet6RouteExcludeAddress, prefix)
					}
				}
			}
		}
		monitor.Start("open interface")
		if t.platformInterface != nil && t.platformInterface.UsePlatformInterface() {
			tunInterface, err = t.platformInterface.OpenInterface(&tunOptions, t.platformOptions)
		} else {
			tunInterface, err = tun.New(tunOptions)
		}
		monitor.Finish()
		t.tunOptions.Name = tunOptions.Name
		if err != nil {
			return E.Cause(err, "configure tun interface")
		}
		t.logger.Trace("creating stack")
		t.tunIf = tunInterface
		if t.platformInterface != nil {
			err = t.platformInterface.ProcessPlatformOptions(t.platformOptions)
			if err != nil {
				closeError := t.tunIf.Close()
				t.tunIf = nil
				return E.Errors(E.Cause(err, "process platform options"), closeError)
			}
		}
		var includeAllNetworks bool
		if t.platformInterface != nil && t.platformInterface.UnderNetworkExtension() {
			includeAllNetworks = t.platformInterface.NetworkExtensionIncludeAllNetworks()
		}
		tunStack, err := tun.NewStack(t.stack, tun.StackOptions{
			Context:                t.ctx,
			Tun:                    tunInterface,
			TunOptions:             t.tunOptions,
			UDPTimeout:             t.udpTimeout,
			ICMPTimeout:            C.ICMPTimeout,
			UDPMapping:             t.udpMapping,
			UDPFiltering:           t.udpFiltering,
			UDPNATMax:              t.udpNATMax,
			Handler:                t,
			Logger:                 t.logger,
			ForwarderBindInterface: C.IsDarwin,
			InterfaceFinder:        t.networkManager.InterfaceFinder(),
			IncludeAllNetworks:     includeAllNetworks,
		})
		if err != nil {
			return err
		}
		t.tunStack = tunStack
		t.logger.Info("started at ", t.tunOptions.Name)
	case adapter.StartStatePostStart:
		monitor := taskmonitor.New(t.logger, C.StartTimeout)
		monitor.Start("starting tun stack")
		err := t.tunStack.Start()
		monitor.Finish()
		if err != nil {
			return E.Cause(err, "starting tun stack")
		}
		monitor.Start("starting tun interface")
		err = t.tunIf.Start()
		monitor.Finish()
		if err != nil {
			return E.Cause(err, "starting TUN interface")
		}
		if t.autoRedirect != nil {
			monitor.Start("initialize auto-redirect")
			err := t.autoRedirect.Start()
			monitor.Finish()
			if err != nil {
				return E.Cause(err, "auto-redirect")
			}
		}
	}
	return nil
}

func (t *Inbound) updateRouteAddressSet(it adapter.RuleSet) {
	routeAddressSet := common.FlatMap(t.routeRuleSet, adapter.RuleSet.ExtractIPSet)
	routeExcludeAddressSet := common.FlatMap(t.routeExcludeRuleSet, adapter.RuleSet.ExtractIPSet)
	t.routeAddressSetAccess.Lock()
	t.routeAddressSet = routeAddressSet
	t.routeExcludeAddressSet = routeExcludeAddressSet
	t.routeAddressSetAccess.Unlock()
	err := t.autoRedirect.UpdateRouteAddressSet()
	if err != nil {
		t.logger.Error("update route address set: ", err)
	}
}

//nolint:unused
func (t *Inbound) routeAddressSetPrefixes() (include []netip.Prefix, exclude []netip.Prefix) {
	t.routeAddressSetAccess.RLock()
	defer t.routeAddressSetAccess.RUnlock()
	include = common.FlatMap(t.routeAddressSet, (*netipx.IPSet).Prefixes)
	exclude = common.FlatMap(t.routeExcludeAddressSet, (*netipx.IPSet).Prefixes)
	return
}

func (t *Inbound) InterfaceUpdated(ctx context.Context) {
	tunStack := t.tunStack
	if tunStack != nil {
		tunStack.ResetNetwork()
	}
}

func (t *Inbound) Close() error {
	return common.Close(
		t.tunStack,
		t.tunIf,
		t.autoRedirect,
	)
}

func (t *Inbound) JudgeFlow(network uint8, source netip.AddrPort, destination netip.AddrPort, firstPacket []byte) tun.FlowVerdict {
	if slices.Contains(t.dnsHijackAddress, destination.Addr()) {
		if network == uint8(header.UDPProtocolNumber) {
			return tun.FlowVerdict{Action: tun.ActionHijackDNS}
		}
		return tun.FlowVerdict{Action: tun.ActionAccept}
	}
	t.routeAddressSetAccess.RLock()
	routeAddressSet := t.routeAddressSet
	routeExcludeAddressSet := t.routeExcludeAddressSet
	t.routeAddressSetAccess.RUnlock()
	destinationAddress := destination.Addr()
	if len(routeAddressSet) > 0 && !slices.ContainsFunc(routeAddressSet, func(it *netipx.IPSet) bool {
		return it.Contains(destinationAddress)
	}) {
		return tun.FlowVerdict{Action: tun.ActionBypass}
	}
	if slices.ContainsFunc(routeExcludeAddressSet, func(it *netipx.IPSet) bool {
		return it.Contains(destinationAddress)
	}) {
		return tun.FlowVerdict{Action: tun.ActionBypass}
	}
	if t.dnsHijackByPort && destination.Port() == 53 &&
		(network == uint8(header.TCPProtocolNumber) || network == uint8(header.UDPProtocolNumber)) {
		if network == uint8(header.UDPProtocolNumber) {
			return tun.FlowVerdict{Action: tun.ActionHijackDNS}
		}
		return tun.FlowVerdict{Action: tun.ActionAccept}
	}
	return adapter.JudgeFlow(t.router, t.tag, C.TypeTun, network, source, destination, firstPacket)
}

func (t *Inbound) isDNSHijackDestination(destination M.Socksaddr) bool {
	return slices.Contains(t.dnsHijackAddress, destination.Addr) || t.dnsHijackByPort && destination.Port == 53
}

func (t *Inbound) NewDNSPacket(payload []byte, source M.Socksaddr, destination M.Socksaddr, writer N.PacketWriter) {
	ctx := log.ContextWithNewID(t.ctx)
	var metadata adapter.InboundContext
	metadata.Inbound = t.tag
	metadata.InboundType = C.TypeTun
	metadata.Network = N.NetworkUDP
	metadata.Source = source
	metadata.Destination = destination
	metadata.Protocol = C.ProtocolDNS
	t.logger.InfoContext(ctx, "inbound DNS packet from ", source)
	t.router.HijackDNSPacket(ctx, payload, writer, metadata)
}

func (t *Inbound) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	ctx = log.ContextWithNewID(ctx)
	var metadata adapter.InboundContext
	metadata.Inbound = t.tag
	metadata.InboundType = C.TypeTun
	metadata.Source = source
	metadata.Destination = destination
	if t.isDNSHijackDestination(destination) {
		metadata.Protocol = C.ProtocolDNS
	}
	if metadata.Protocol == C.ProtocolDNS {
		t.logger.InfoContext(ctx, "inbound DNS connection from ", metadata.Source)
	} else {
		t.logger.InfoContext(ctx, "inbound connection from ", metadata.Source)
		t.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
	}
	t.router.RouteConnectionEx(ctx, conn, metadata, onClose)
}

func (t *Inbound) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	ctx = log.ContextWithNewID(ctx)
	var metadata adapter.InboundContext
	metadata.Inbound = t.tag
	metadata.InboundType = C.TypeTun
	metadata.Source = source
	metadata.Destination = destination
	if t.isDNSHijackDestination(destination) {
		metadata.Protocol = C.ProtocolDNS
	}
	if metadata.Protocol == C.ProtocolDNS {
		t.logger.InfoContext(ctx, "inbound DNS packet connection from ", metadata.Source)
	} else {
		t.logger.InfoContext(ctx, "inbound packet connection from ", metadata.Source)
		t.logger.InfoContext(ctx, "inbound packet connection to ", metadata.Destination)
	}
	t.router.RoutePacketConnectionEx(ctx, conn, metadata, onClose)
}

type autoRedirectHandler Inbound

func (t *autoRedirectHandler) JudgeFlow(network uint8, source netip.AddrPort, destination netip.AddrPort, firstPacket []byte) tun.FlowVerdict {
	return (*Inbound)(t).JudgeFlow(network, source, destination, firstPacket)
}

func (t *autoRedirectHandler) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	ctx = log.ContextWithNewID(ctx)
	var metadata adapter.InboundContext
	metadata.Inbound = t.tag
	metadata.InboundType = C.TypeTun
	metadata.Source = source
	metadata.Destination = destination
	if (*Inbound)(t).isDNSHijackDestination(destination) {
		metadata.Protocol = C.ProtocolDNS
	}
	if metadata.Protocol == C.ProtocolDNS {
		t.logger.InfoContext(ctx, "inbound redirect DNS connection from ", metadata.Source)
	} else {
		t.logger.InfoContext(ctx, "inbound redirect connection from ", metadata.Source)
		t.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
	}
	t.router.RouteConnectionEx(ctx, conn, metadata, onClose)
}

var _ tun.AutoRedirectHandler = (*autoRedirectHandler)(nil)
