package bridge

import (
	"net"
	"net/netip"
	"slices"
	"strconv"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"

	"golang.org/x/sys/unix"
)

func buildBridgeAnchorRules(ruleLogger logger.ContextLogger, tunName string, egress string, boundInterface string, inet4Port netip.Addr, inet6Port netip.Addr) []pfAnchorRule {
	egressInterface, err := net.InterfaceByName(egress)
	if err != nil {
		return nil
	}
	mtu := egressInterface.MTU
	if mtu < 576 || mtu > bridgeTunMTU {
		mtu = bridgeTunMTU
	}
	localPrefixes, inet4Interfaces, inet6Interfaces := collectLocalSegments(egress, boundInterface, inet4Port.IsValid(), inet6Port.IsValid())
	var rules []pfAnchorRule
	if inet4Port.IsValid() {
		rules = append(rules, pfScrubRule(egress, inet4Port, uint16(mtu-40)))
	}
	if inet6Port.IsValid() {
		rules = append(rules, pfScrubRule(egress, inet6Port, uint16(mtu-60)))
	}
	if inet4Port.IsValid() {
		rules = append(rules, pfNatRule(egress, inet4Port))
		for _, name := range inet4Interfaces {
			rules = append(rules, pfNatRule(name, inet4Port))
		}
	}
	if inet6Port.IsValid() {
		rules = append(rules, pfNatRule(egress, inet6Port))
		for _, name := range inet6Interfaces {
			rules = append(rules, pfNatRule(name, inet6Port))
		}
	}
	// pf evaluates translation on the interface the routing table picks, and
	// route-to on an out rule does not re-run it on the new interface: when
	// another tun holds the default route the nat-on-egress rule never matches.
	// route-to on the in side redirects before routing, so the packet actually
	// leaves via the egress and the nat rule applies there.
	if inet4Port.IsValid() {
		gateway := interfaceGateway(egressInterface.Index, true)
		if gateway.IsValid() {
			rules = append(rules, pfRouteToRule(tunName, egress, gateway, inet4Port))
		} else {
			ruleLogger.Debug("no IPv4 gateway on ", egress, ", relying on the default route")
		}
	}
	if inet6Port.IsValid() {
		gateway := interfaceGateway(egressInterface.Index, false)
		if gateway.IsValid() {
			rules = append(rules, pfRouteToRule(tunName, egress, gateway, inet6Port))
		} else {
			ruleLogger.Debug("no IPv6 gateway on ", egress, ", relying on the default route")
		}
	}
	// pf rules are last-match: the pass rules below override the route-to pin
	// for destinations in connected subnets, so the routing table delivers them
	// on their own interface.
	for _, prefix := range localPrefixes {
		port := inet4Port
		if !prefix.Addr().Is4() {
			port = inet6Port
		}
		rules = append(rules, pfPassInRule(tunName, port, prefix))
	}
	return rules
}

// collectLocalSegments returns the connected subnets whose destinations bypass
// the route-to pin so the routing table delivers them on their own interface,
// plus the non-egress interfaces that then need their own masquerade rule.
// With a pinned egress only its own subnets bypass, matching the Linux backend.
func collectLocalSegments(egress string, boundInterface string, inet4Active bool, inet6Active bool) (prefixes []netip.Prefix, inet4Interfaces []string, inet6Interfaces []string) {
	localInterfaces, err := net.Interfaces()
	if err != nil {
		return
	}
	for _, localInterface := range localInterfaces {
		if boundInterface != "" && localInterface.Name != boundInterface {
			continue
		}
		if localInterface.Flags&net.FlagUp == 0 || localInterface.Flags&net.FlagBroadcast == 0 ||
			localInterface.Flags&net.FlagLoopback != 0 || localInterface.Flags&net.FlagPointToPoint != 0 {
			continue
		}
		interfaceAddrs, addrsErr := localInterface.Addrs()
		if addrsErr != nil {
			continue
		}
		var (
			hasInet4 bool
			hasInet6 bool
		)
		for _, interfaceAddr := range interfaceAddrs {
			ipNet, isIPNet := interfaceAddr.(*net.IPNet)
			if !isIPNet {
				continue
			}
			address, valid := netip.AddrFromSlice(ipNet.IP)
			if !valid {
				continue
			}
			address = address.Unmap()
			if address.IsLinkLocalUnicast() {
				continue
			}
			if address.Is4() {
				if !inet4Active {
					continue
				}
				hasInet4 = true
			} else {
				if !inet6Active {
					continue
				}
				hasInet6 = true
			}
			bits, _ := ipNet.Mask.Size()
			prefix := netip.PrefixFrom(address, bits).Masked()
			if !slices.Contains(prefixes, prefix) {
				prefixes = append(prefixes, prefix)
			}
		}
		if localInterface.Name == egress {
			continue
		}
		if hasInet4 {
			inet4Interfaces = append(inet4Interfaces, localInterface.Name)
		}
		if hasInet6 {
			inet6Interfaces = append(inet6Interfaces, localInterface.Name)
		}
	}
	return
}

func pfScrubRule(egress string, port netip.Addr, maxMSS uint16) pfAnchorRule {
	rule := pfRule{
		Action: pfActionScrub,
		AF:     pfFamily(port.Is4()),
		Proto:  unix.IPPROTO_TCP,
		MaxMSS: maxMSS,
	}
	copy(rule.IfName[:], egress)
	rule.Src.Addr = pfHostAddress(port)
	return pfAnchorRule{RulesetIndex: pfRulesetScrub, Rule: rule}
}

func pfNatRule(interfaceName string, port netip.Addr) pfAnchorRule {
	rule := pfRule{
		Action: pfActionNat,
		AF:     pfFamily(port.Is4()),
	}
	rule.RPool.ProxyPort = [2]uint16{pfNatProxyPortLow, pfNatProxyPortHigh}
	copy(rule.IfName[:], interfaceName)
	rule.Src.Addr = pfHostAddress(port)
	return pfAnchorRule{
		RulesetIndex: pfRulesetNat,
		Rule:         rule,
		Pool:         pfPoolAddr{Addr: pfDynamicInterfaceAddress(interfaceName, port.Is4())},
	}
}

func pfPassInRule(tunName string, port netip.Addr, destination netip.Prefix) pfAnchorRule {
	rule := pfRule{
		Action:    pfActionPass,
		Direction: pfDirectionIn,
		AF:        pfFamily(port.Is4()),
		KeepState: pfStateNormal,
	}
	copy(rule.IfName[:], tunName)
	rule.Src.Addr = pfHostAddress(port)
	if destination.IsValid() {
		rule.Dst.Addr = pfPrefixAddress(destination)
	}
	return pfAnchorRule{RulesetIndex: pfRulesetFilter, Rule: rule}
}

func pfRouteToRule(tunName string, egress string, gateway netip.Addr, port netip.Addr) pfAnchorRule {
	anchorRule := pfPassInRule(tunName, port, netip.Prefix{})
	anchorRule.Rule.RouteAction = pfRouteActionRouteTo
	anchorRule.Pool = pfPoolAddr{Addr: pfHostAddress(gateway)}
	copy(anchorRule.Pool.IfName[:], egress)
	return anchorRule
}

// Assigning the port as the utun's point-to-point destination makes the kernel
// install the host route itself; a plain interface route against an address-less
// utun fails with ENETUNREACH.
func assignBridgePortAddress(tunName string, local netip.Addr, port netip.Addr) error {
	if !port.IsValid() {
		return nil
	}
	err := assignPointToPointAddress(tunName, local, port)
	if err != nil {
		return E.Cause(err, "assign bridge address")
	}
	err = addInterfaceHostRoute(port, tunName)
	if err != nil {
		return E.Cause(err, "add bridge host route")
	}
	return nil
}

func enableDarwinForwarding(forwardingLogger logger.ContextLogger, inet4Active bool, inet6Active bool) []sysctlState {
	var restore []sysctlState
	enable := func(name string) {
		mib := forwardingMibs[name]
		value, err := getSysctlInt32(mib)
		if err != nil {
			forwardingLogger.Debug(E.Cause(err, "read ", name))
			return
		}
		if value == 1 {
			return
		}
		err = setSysctlInt32(mib, 1)
		if err != nil {
			forwardingLogger.Debug(E.Cause(err, "enable ", name))
			return
		}
		restore = append(restore, sysctlState{name: name, value: strconv.Itoa(int(value))})
	}
	if inet4Active {
		enable("net.inet.ip.forwarding")
	}
	if inet6Active {
		enable("net.inet6.ip6.forwarding")
	}
	return restore
}

func restoreDarwinForwarding(states []sysctlState) {
	for _, state := range states {
		value, err := strconv.Atoi(state.value)
		if err != nil {
			continue
		}
		_ = setSysctlInt32(forwardingMibs[state.name], int32(value))
	}
}
