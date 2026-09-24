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
	err = setupBridgeFamily(s.tunName, s.ruleIndex, s.routeTable, false, unix.AF_INET, s.inet4Port)
	if err != nil {
		return E.Cause(err, "set up bridge routing")
	}
	err = setupBridgeFamily(s.tunName, s.ruleIndex, s.routeTable, false, unix.AF_INET6, s.inet6Port)
	if err != nil {
		s.logger.Debug(E.Cause(err, "IPv6 bridge routing unavailable, disabling IPv6 forwarding"))
		removeBridgeFamily(s.tunName, s.ruleIndex, s.routeTable, false, unix.AF_INET6, s.inet6Port)
		s.inet6Port = netip.Addr{}
	}
	for _, family := range activeBridgeFamilies(s.inet6Port) {
		unroutableBridgeDefault(s.routeTable, family, unix.RTN_BLACKHOLE)
	}
	s.startNetworkMonitor()
	return nil
}

func (s *Service) syncEgressLocked() error {
	var link netlink.Link
	if s.egressName != "" {
		link, _ = netlink.LinkByName(s.egressName)
	}
	var routes []netlink.Route
	for _, family := range activeBridgeFamilies(s.inet6Port) {
		if link == nil {
			routes = append(routes, unroutableBridgeRoute(s.routeTable, family, unix.RTN_BLACKHOLE))
		} else {
			routes = append(routes, egressRoutes(s.routeTable, family, link, unix.RTN_BLACKHOLE)...)
		}
	}
	if link != nil {
		s.updateClampLocked(link.Attrs().MTU)
	}
	if bridgeRoutesEqual(listBridgeRoutes(s.routeTable), routes) {
		return nil
	}
	if s.egressName != "" && link == nil {
		s.logger.Debug("bridge egress ", s.egressName, " absent, dropping forwarded traffic")
	}
	err := applyBridgeRoutes(s.routeTable, routes, unix.RTN_BLACKHOLE)
	if err != nil {
		s.logger.Debug(E.Cause(err, "pin bridge egress default route"))
	}
	return nil
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
		removeBridgeFamily(s.tunName, s.ruleIndex, s.routeTable, false, unix.AF_INET, s.inet4Port)
		removeBridgeFamily(s.tunName, s.ruleIndex, s.routeTable, false, unix.AF_INET6, s.inet6Port)
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
