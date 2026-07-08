package bridge

import (
	"net"
	"net/netip"
	_ "unsafe"

	"github.com/sagernet/netlink"
	"github.com/sagernet/sing-tun"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"

	"golang.org/x/sys/unix"
)

type ServiceOptions struct {
	BridgeName string
	MTU        int
	Inet4Port  netip.Addr
	Inet6Port  netip.Addr
	RuleIndex  int
	RouteTable int
	Logger     logger.ContextLogger
}

type Service struct {
	serviceBase

	ruleIndex    int
	routeTable   int
	nftTableName string
	clampMTU     int
}

func NewService(options ServiceOptions) (*Service, error) {
	if !options.Inet4Port.IsValid() {
		return nil, E.New("missing bridge IPv4 port address")
	}
	if options.RouteTable == 0 {
		return nil, E.New("missing bridge route table index")
	}
	serviceLogger := options.Logger
	if serviceLogger == nil {
		serviceLogger = logger.NOP()
	}
	instance := &Service{
		serviceBase: serviceBase{
			logger:            serviceLogger,
			mtu:               options.MTU,
			inet4Port:         options.Inet4Port,
			inet6Port:         options.Inet6Port,
			tunFileDescriptor: -1,
		},
		ruleIndex:  options.RuleIndex,
		routeTable: options.RouteTable,
	}
	instance.applyEgress = instance.syncEgressLocked
	err := instance.start(options.BridgeName)
	if err != nil {
		instance.Close()
		return nil, err
	}
	return instance, nil
}

func (s *Service) start(bridgeName string) error {
	s.tunName = tun.CalculateInterfaceName(bridgeName)
	s.nftTableName = "sing-box-" + s.tunName
	tunFileDescriptor, err := openTUN(s.tunName, true)
	if err != nil {
		return E.Cause(err, "create bridge tun")
	}
	err = setTCPOffload(tunFileDescriptor)
	if err != nil {
		s.logger.Warn(E.Cause(err, "set TCP offload"))
	}
	err = setUDPOffload(tunFileDescriptor)
	if err != nil {
		s.logger.Warn(E.Cause(err, "set UDP offload"))
	}
	s.tunFileDescriptor = tunFileDescriptor
	tunLink, err := netlink.LinkByName(s.tunName)
	if err != nil {
		return E.Cause(err, "find bridge tun")
	}
	err = netlink.LinkSetMTU(tunLink, s.mtu)
	if err != nil {
		return E.Cause(err, "set bridge tun mtu")
	}
	err = netlink.LinkSetUp(tunLink)
	if err != nil {
		return E.Cause(err, "set bridge tun up")
	}
	inet6Active, err := setupBridgeNetfilter(s.logger, s.nftTableName, s.tunName, s.inet6Port.IsValid())
	if err != nil {
		return E.Cause(err, "set up bridge netfilter")
	}
	if !inet6Active {
		s.inet6Port = netip.Addr{}
	}
	s.forwardingRestore = enableBridgeForwarding(s.logger, s.tunName, s.inet4Port.IsValid(), s.inet6Port.IsValid())
	err = setupBridgeFamily(s.tunName, s.ruleIndex, s.routeTable, unix.AF_INET, s.inet4Port)
	if err != nil {
		return E.Cause(err, "set up bridge routing")
	}
	err = setupBridgeFamily(s.tunName, s.ruleIndex, s.routeTable, unix.AF_INET6, s.inet6Port)
	if err != nil {
		s.logger.Debug(E.Cause(err, "IPv6 bridge routing unavailable, disabling IPv6 forwarding"))
		removeBridgeFamily(s.tunName, s.ruleIndex, s.routeTable, unix.AF_INET6, s.inet6Port)
		s.inet6Port = netip.Addr{}
	}
	for _, family := range activeBridgeFamilies(s.inet6Port) {
		blackholeBridgeDefault(s.routeTable, family)
	}
	s.startNetworkMonitor()
	return nil
}

func (s *Service) syncEgressLocked() {
	flushBridgeRouteTable(s.routeTable)
	if s.egressName == "" {
		for _, family := range activeBridgeFamilies(s.inet6Port) {
			blackholeBridgeDefault(s.routeTable, family)
		}
		return
	}
	link, err := netlink.LinkByName(s.egressName)
	if err != nil {
		for _, family := range activeBridgeFamilies(s.inet6Port) {
			blackholeBridgeDefault(s.routeTable, family)
		}
		s.logger.Debug("bridge egress ", s.egressName, " absent, dropping forwarded traffic")
		return
	}
	for _, family := range activeBridgeFamilies(s.inet6Port) {
		s.syncEgressFamilyLocked(family, link.Attrs().Index)
	}
	s.updateClampLocked(link.Attrs().MTU)
}

// Unlike the in-process backend this copies routes from every table: on Android
// netd leaves the main table empty and keeps each network's routes in its own
// table, resolvable only through fwmark rules that forwarded packets never carry.
func (s *Service) syncEgressFamilyLocked(family int, linkIndex int) {
	routes, err := netlink.RouteListFiltered(family, &netlink.Route{
		LinkIndex: linkIndex,
		Table:     unix.RT_TABLE_UNSPEC,
	}, netlink.RT_FILTER_OIF|netlink.RT_FILTER_TABLE)
	if err != nil {
		blackholeBridgeDefault(s.routeTable, family)
		return
	}
	var defaultRoute *netlink.Route
	for _, route := range routes {
		if route.Table == unix.RT_TABLE_LOCAL || route.Table == s.routeTable {
			continue
		}
		if route.Type != unix.RTN_UNICAST {
			continue
		}
		if isDefaultDestination(route.Dst) {
			if defaultRoute == nil {
				pinned := route
				defaultRoute = &pinned
			}
			continue
		}
		if route.Gw != nil {
			continue
		}
		connected := route
		connected.Table = s.routeTable
		connected.ILinkIndex = 0
		_ = netlink.RouteReplace(&connected)
	}
	if defaultRoute == nil {
		blackholeBridgeDefault(s.routeTable, family)
		s.logger.Debug("no default route on bridge egress ", s.egressName)
		return
	}
	defaultRoute.Table = s.routeTable
	defaultRoute.ILinkIndex = 0
	err = netlink.RouteReplace(defaultRoute)
	if err != nil {
		blackholeBridgeDefault(s.routeTable, family)
		s.logger.Debug(E.Cause(err, "pin bridge egress default route"))
	}
}

func (s *Service) updateClampLocked(egressMTU int) {
	mtu := s.mtu
	if egressMTU >= 576 && egressMTU < mtu {
		mtu = egressMTU
	}
	if mtu == s.clampMTU {
		return
	}
	err := setupBridgeClamp(s.nftTableName, s.tunName, s.inet4Port, s.inet6Port, mtu)
	if err != nil {
		s.logger.Debug(E.Cause(err, "update bridge MSS clamp"))
		return
	}
	s.clampMTU = mtu
}

func (s *Service) Close() error {
	if !s.beginClose() {
		return nil
	}
	s.access.Lock()
	defer s.access.Unlock()
	if s.tunName != "" {
		cleanupBridgeNetfilter(s.nftTableName)
		removeBridgeFamily(s.tunName, s.ruleIndex, s.routeTable, unix.AF_INET, s.inet4Port)
		removeBridgeFamily(s.tunName, s.ruleIndex, s.routeTable, unix.AF_INET6, s.inet6Port)
		flushBridgeRouteTable(s.routeTable)
	}
	restoreBridgeForwarding(s.forwardingRestore)
	s.forwardingRestore = nil
	if s.tunFileDescriptor >= 0 {
		_ = unix.Close(s.tunFileDescriptor)
		s.tunFileDescriptor = -1
	}
	return nil
}

func isDefaultDestination(destination *net.IPNet) bool {
	if destination == nil {
		return true
	}
	ones, _ := destination.Mask.Size()
	return ones == 0
}

//go:linkname openTUN github.com/sagernet/sing-tun.open
func openTUN(name string, vnetHdr bool) (int, error)

//go:linkname setTCPOffload github.com/sagernet/sing-tun.setTCPOffload
func setTCPOffload(fd int) error

//go:linkname setUDPOffload github.com/sagernet/sing-tun.setUDPOffload
func setUDPOffload(fd int) error
