package tun

import (
	"context"
	"net"
	"net/netip"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/common/taskmonitor"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/deprecated"
	"github.com/sagernet/sing-box/experimental/libbox/platform"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/route/rule"
	"github.com/sagernet/sing-tun"
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
	tag            string
	ctx            context.Context
	router         adapter.Router
	networkManager adapter.NetworkManager
	logger         log.ContextLogger
	//nolint:staticcheck
	inboundOptions              option.InboundOptions
	tunOptions                  tun.Options
	udpTimeout                  time.Duration
	stack                       string
	tunIf                       tun.Tun
	tunStack                    tun.Stack
	platformInterface           platform.Interface
	platformOptions             option.TunPlatformOptions
	autoRedirect                tun.AutoRedirect
	routeRuleSet                []adapter.RuleSet
	routeRuleSetCallback        []*list.Element[adapter.RuleSetUpdateCallback]
	routeExcludeRuleSet         []adapter.RuleSet
	routeExcludeRuleSetCallback []*list.Element[adapter.RuleSetUpdateCallback]
	routeAddressSet             []*netipx.IPSet
	routeExcludeAddressSet      []*netipx.IPSet
}

func NewInbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.TunInboundOptions) (adapter.Inbound, error) {
	address := options.Address
	var deprecatedAddressUsed bool

	//nolint:staticcheck
	if len(options.Inet4Address) > 0 {
		address = append(address, options.Inet4Address...)
		deprecatedAddressUsed = true
	}

	//nolint:staticcheck
	if len(options.Inet6Address) > 0 {
		address = append(address, options.Inet6Address...)
		deprecatedAddressUsed = true
	}
	inet4Address := common.Filter(address, func(it netip.Prefix) bool {
		return it.Addr().Is4()
	})
	inet6Address := common.Filter(address, func(it netip.Prefix) bool {
		return it.Addr().Is6()
	})

	routeAddress := options.RouteAddress

	//nolint:staticcheck
	if len(options.Inet4RouteAddress) > 0 {
		routeAddress = append(routeAddress, options.Inet4RouteAddress...)
		deprecatedAddressUsed = true
	}

	//nolint:staticcheck
	if len(options.Inet6RouteAddress) > 0 {
		routeAddress = append(routeAddress, options.Inet6RouteAddress...)
		deprecatedAddressUsed = true
	}
	inet4RouteAddress := common.Filter(routeAddress, func(it netip.Prefix) bool {
		return it.Addr().Is4()
	})
	inet6RouteAddress := common.Filter(routeAddress, func(it netip.Prefix) bool {
		return it.Addr().Is6()
	})

	routeExcludeAddress := options.RouteExcludeAddress

	//nolint:staticcheck
	if len(options.Inet4RouteExcludeAddress) > 0 {
		routeExcludeAddress = append(routeExcludeAddress, options.Inet4RouteExcludeAddress...)
		deprecatedAddressUsed = true
	}

	//nolint:staticcheck
	if len(options.Inet6RouteExcludeAddress) > 0 {
		routeExcludeAddress = append(routeExcludeAddress, options.Inet6RouteExcludeAddress...)
		deprecatedAddressUsed = true
	}
	inet4RouteExcludeAddress := common.Filter(routeExcludeAddress, func(it netip.Prefix) bool {
		return it.Addr().Is4()
	})
	inet6RouteExcludeAddress := common.Filter(routeExcludeAddress, func(it netip.Prefix) bool {
		return it.Addr().Is6()
	})

	if deprecatedAddressUsed {
		deprecated.Report(ctx, deprecated.OptionTUNAddressX)
	}

	//nolint:staticcheck
	if options.GSO {
		deprecated.Report(ctx, deprecated.OptionTUNGSO)
	}

	platformInterface := service.FromContext[platform.Interface](ctx)
	tunMTU := options.MTU
	enableGSO := C.IsLinux && options.Stack == "gvisor" && platformInterface == nil && tunMTU > 0 && tunMTU < 49152
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
	inputMark := uint32(options.AutoRedirectInputMark)
	if inputMark == 0 {
		inputMark = tun.DefaultAutoRedirectInputMark
	}
	outputMark := uint32(options.AutoRedirectOutputMark)
	if outputMark == 0 {
		outputMark = tun.DefaultAutoRedirectOutputMark
	}
	networkManager := service.FromContext[adapter.NetworkManager](ctx)
	multiPendingPackets := C.IsDarwin && ((options.Stack == "gvisor" && tunMTU < 32768) || (options.Stack != "gvisor" && options.MTU <= 9000))
	inbound := &Inbound{
		tag:            tag,
		ctx:            ctx,
		router:         router,
		networkManager: networkManager,
		logger:         logger,
		inboundOptions: options.InboundOptions,
		tunOptions: tun.Options{
			Name:                     options.InterfaceName,
			MTU:                      tunMTU,
			GSO:                      enableGSO,
			Inet4Address:             inet4Address,
			Inet6Address:             inet6Address,
			AutoRoute:                options.AutoRoute,
			IPRoute2TableIndex:       tableIndex,
			IPRoute2RuleIndex:        ruleIndex,
			AutoRedirectInputMark:    inputMark,
			AutoRedirectOutputMark:   outputMark,
			ExcludeMPTCP:             options.ExcludeMPTCP,
			Inet4LoopbackAddress:     common.Filter(options.LoopbackAddress, netip.Addr.Is4),
			Inet6LoopbackAddress:     common.Filter(options.LoopbackAddress, netip.Addr.Is6),
			StrictRoute:              options.StrictRoute,
			IncludeInterface:         options.IncludeInterface,
			ExcludeInterface:         options.ExcludeInterface,
			Inet4RouteAddress:        inet4RouteAddress,
			Inet6RouteAddress:        inet6RouteAddress,
			Inet4RouteExcludeAddress: inet4RouteExcludeAddress,
			Inet6RouteExcludeAddress: inet6RouteExcludeAddress,
			IncludeUID:               includeUID,
			ExcludeUID:               excludeUID,
			IncludeAndroidUser:       options.IncludeAndroidUser,
			IncludePackage:           options.IncludePackage,
			ExcludePackage:           options.ExcludePackage,
			InterfaceMonitor:         networkManager.InterfaceMonitor(),
			EXP_MultiPendingPackets:  multiPendingPackets,
		},
		udpTimeout:        udpTimeout,
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
		disableNFTables, dErr := strconv.ParseBool(os.Getenv("DISABLE_NFTABLES"))
		inbound.autoRedirect, err = tun.NewAutoRedirect(tun.AutoRedirectOptions{
			TunOptions:             &inbound.tunOptions,
			Context:                ctx,
			Handler:                (*autoRedirectHandler)(inbound),
			Logger:                 logger,
			NetworkMonitor:         networkManager.NetworkMonitor(),
			InterfaceFinder:        networkManager.InterfaceFinder(),
			TableName:              "sing-box",
			DisableNFTables:        dErr == nil && disableNFTables,
			RouteAddressSet:        &inbound.routeAddressSet,
			RouteExcludeAddressSet: &inbound.routeExcludeAddressSet,
		})
		if err != nil {
			return nil, E.Cause(err, "initialize auto-redirect")
		}
		if !C.IsAndroid {
			inbound.tunOptions.AutoRedirectMarkMode = true
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
	case adapter.StartStateStart:
		if C.IsAndroid && t.platformInterface == nil {
			t.tunOptions.BuildAndroidRules(t.networkManager.PackageManager())
		}
		if t.tunOptions.Name == "" {
			t.tunOptions.Name = tun.CalculateInterfaceName("")
		}
		if t.platformInterface == nil {
			t.routeAddressSet = common.FlatMap(t.routeRuleSet, adapter.RuleSet.ExtractIPSet)
			for _, routeRuleSet := range t.routeRuleSet {
				ipSets := routeRuleSet.ExtractIPSet()
				if len(ipSets) == 0 {
					t.logger.Warn("route_address_set: no destination IP CIDR rules found in rule-set: ", routeRuleSet.Name())
				}
				routeRuleSet.IncRef()
				t.routeAddressSet = append(t.routeAddressSet, ipSets...)
				if t.autoRedirect != nil {
					t.routeRuleSetCallback = append(t.routeRuleSetCallback, routeRuleSet.RegisterCallback(t.updateRouteAddressSet))
				}
			}
			t.routeExcludeAddressSet = common.FlatMap(t.routeExcludeRuleSet, adapter.RuleSet.ExtractIPSet)
			for _, routeExcludeRuleSet := range t.routeExcludeRuleSet {
				ipSets := routeExcludeRuleSet.ExtractIPSet()
				if len(ipSets) == 0 {
					t.logger.Warn("route_address_set: no destination IP CIDR rules found in rule-set: ", routeExcludeRuleSet.Name())
				}
				routeExcludeRuleSet.IncRef()
				t.routeExcludeAddressSet = append(t.routeExcludeAddressSet, ipSets...)
				if t.autoRedirect != nil {
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
			for _, ipSet := range t.routeAddressSet {
				for _, prefix := range ipSet.Prefixes() {
					if prefix.Addr().Is4() {
						tunOptions.Inet4RouteAddress = append(tunOptions.Inet4RouteAddress, prefix)
					} else {
						tunOptions.Inet6RouteAddress = append(tunOptions.Inet6RouteAddress, prefix)
					}
				}
			}
			for _, ipSet := range t.routeExcludeAddressSet {
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
		if t.platformInterface != nil {
			tunInterface, err = t.platformInterface.OpenTun(&tunOptions, t.platformOptions)
		} else {
			if HookBeforeCreatePlatformInterface != nil {
				HookBeforeCreatePlatformInterface()
			}
			tunInterface, err = tun.New(tunOptions)
		}
		monitor.Finish()
		t.tunOptions.Name = tunOptions.Name
		if err != nil {
			return E.Cause(err, "configure tun interface")
		}
		t.logger.Trace("creating stack")
		t.tunIf = tunInterface
		var (
			forwarderBindInterface bool
			includeAllNetworks     bool
		)
		if t.platformInterface != nil {
			forwarderBindInterface = true
			includeAllNetworks = t.platformInterface.IncludeAllNetworks()
		}
		tunStack, err := tun.NewStack(t.stack, tun.StackOptions{
			Context:                t.ctx,
			Tun:                    tunInterface,
			TunOptions:             t.tunOptions,
			UDPTimeout:             t.udpTimeout,
			Handler:                t,
			Logger:                 t.logger,
			ForwarderBindInterface: forwarderBindInterface,
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
		t.routeAddressSet = nil
		t.routeExcludeAddressSet = nil
	}
	return nil
}

func (t *Inbound) updateRouteAddressSet(it adapter.RuleSet) {
	t.routeAddressSet = common.FlatMap(t.routeRuleSet, adapter.RuleSet.ExtractIPSet)
	t.routeExcludeAddressSet = common.FlatMap(t.routeExcludeRuleSet, adapter.RuleSet.ExtractIPSet)
	t.autoRedirect.UpdateRouteAddressSet()
	t.routeAddressSet = nil
	t.routeExcludeAddressSet = nil
}

func (t *Inbound) Close() error {
	return common.Close(
		t.tunStack,
		t.tunIf,
		t.autoRedirect,
	)
}

func (t *Inbound) PrepareConnection(network string, source M.Socksaddr, destination M.Socksaddr, routeContext tun.DirectRouteContext, timeout time.Duration) (tun.DirectRouteDestination, error) {
	var ipVersion uint8
	if !destination.IsIPv6() {
		ipVersion = 4
	} else {
		ipVersion = 6
	}
	routeDestination, err := t.router.PreMatch(adapter.InboundContext{
		Inbound:        t.tag,
		InboundType:    C.TypeTun,
		IPVersion:      ipVersion,
		Network:        network,
		Source:         source,
		Destination:    destination,
		InboundOptions: t.inboundOptions,
	}, routeContext, timeout)
	if err != nil {
		if !rule.IsRejected(err) {
			t.logger.Warn(E.Cause(err, "link ", network, " connection from ", source.AddrString(), " to ", destination.AddrString()))
		}
	}
	return routeDestination, err
}

func (t *Inbound) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	ctx = log.ContextWithNewID(ctx)
	var metadata adapter.InboundContext
	metadata.Inbound = t.tag
	metadata.InboundType = C.TypeTun
	metadata.Source = source
	metadata.Destination = destination
	//nolint:staticcheck
	metadata.InboundOptions = t.inboundOptions
	t.logger.InfoContext(ctx, "inbound connection from ", metadata.Source)
	t.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
	t.router.RouteConnectionEx(ctx, conn, metadata, onClose)
}

func (t *Inbound) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	ctx = log.ContextWithNewID(ctx)
	var metadata adapter.InboundContext
	metadata.Inbound = t.tag
	metadata.InboundType = C.TypeTun
	metadata.Source = source
	metadata.Destination = destination
	//nolint:staticcheck
	metadata.InboundOptions = t.inboundOptions
	t.logger.InfoContext(ctx, "inbound packet connection from ", metadata.Source)
	t.logger.InfoContext(ctx, "inbound packet connection to ", metadata.Destination)
	t.router.RoutePacketConnectionEx(ctx, conn, metadata, onClose)
}

type autoRedirectHandler Inbound

func (t *autoRedirectHandler) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	ctx = log.ContextWithNewID(ctx)
	var metadata adapter.InboundContext
	metadata.Inbound = t.tag
	metadata.InboundType = C.TypeTun
	metadata.Source = source
	metadata.Destination = destination
	//nolint:staticcheck
	metadata.InboundOptions = t.inboundOptions
	t.logger.InfoContext(ctx, "inbound redirect connection from ", metadata.Source)
	t.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
	t.router.RouteConnectionEx(ctx, conn, metadata, onClose)
}
