package bridge

import (
	"net"
	"net/netip"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/sagernet/netlink"
	"github.com/sagernet/nftables"
	"github.com/sagernet/nftables/binaryutil"
	"github.com/sagernet/nftables/expr"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"

	"golang.org/x/sys/unix"
)

// fullcone is an out-of-tree nftables verb (the nft_fullcone module), absent on
// stock kernels.
var (
	fullConeProbeOnce   sync.Once
	fullConeProbeResult bool
)

func enableBridgeForwarding(logger logger.ContextLogger, tunName string, inet4 bool, inet6 bool) []sysctlState {
	var restore []sysctlState
	enable := func(path string) bool {
		content, err := os.ReadFile(path)
		if err != nil {
			logger.Debug(E.Cause(err, "read ", path))
			return false
		}
		value := strings.TrimSpace(string(content))
		if value == "1" {
			return false
		}
		err = os.WriteFile(path, []byte("1"), 0o644)
		if err != nil {
			logger.Debug(E.Cause(err, "enable ", path))
			return false
		}
		restore = append(restore, sysctlState{name: path, value: value})
		return true
	}
	if inet4 {
		enable("/proc/sys/net/ipv4/ip_forward")
	}
	if inet6 {
		if enable("/proc/sys/net/ipv6/conf/all/forwarding") {
			restore = append(restore, overruleAcceptRA(logger)...)
		}
	}
	_ = os.WriteFile("/proc/sys/net/ipv4/conf/"+tunName+"/rp_filter", []byte("2"), 0o644)
	return restore
}

// Writing conf/all/forwarding copies forwarding=1 to conf/default and to every
// existing interface (addrconf_fixup_forwarding), and ipv6_accept_ra() then
// requires accept_ra=2 on a forwarding interface; raise interfaces left at the
// host default of 1 so SLAAC (e.g. on PPPoE WANs) survives forwarding.
// conf/default is included so interfaces created afterwards inherit 2.
func overruleAcceptRA(logger logger.ContextLogger) []sysctlState {
	var restore []sysctlState
	entries, err := os.ReadDir("/proc/sys/net/ipv6/conf")
	if err != nil {
		logger.Debug(E.Cause(err, "read /proc/sys/net/ipv6/conf"))
		return nil
	}
	for _, entry := range entries {
		if entry.Name() == "all" {
			continue
		}
		path := "/proc/sys/net/ipv6/conf/" + entry.Name() + "/accept_ra"
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		value := strings.TrimSpace(string(content))
		if value != "1" {
			continue
		}
		err = os.WriteFile(path, []byte("2"), 0o644)
		if err != nil {
			logger.Debug(E.Cause(err, "overrule ", path))
			continue
		}
		restore = append(restore, sysctlState{name: path, value: value})
	}
	return restore
}

func restoreBridgeForwarding(states []sysctlState) {
	for _, state := range states {
		_ = os.WriteFile(state.name, []byte(state.value), 0o644)
	}
}

var (
	nftablesProbeOnce sync.Once
	nftablesMissing   bool
)

// A kernel built without CONFIG_NF_TABLES (common on pre-GKI Android) answers a
// whole nfnetlink batch with a single EOPNOTSUPP ack, while the client waits for
// one ack per batched message and blocks forever; only non-batch requests are
// answered reliably, so probe with a dump before the first batch operation.
func bridgeUseIptables() bool {
	nftablesProbeOnce.Do(func() {
		nft, err := nftables.New()
		if err != nil {
			nftablesMissing = true
			return
		}
		_, err = nft.ListTablesOfFamily(nftables.TableFamilyINet)
		nftablesMissing = err != nil
	})
	return nftablesMissing
}

func setupBridgeNetfilter(logger logger.ContextLogger, tableName string, tunName string, inet6 bool) (bool, error) {
	if bridgeUseIptables() {
		return setupBridgeIptables(logger, tableName, tunName, inet6)
	}
	err := setupBridgeNftables(tableName, tunName)
	if err != nil {
		return false, err
	}
	return inet6, nil
}

func setupBridgeClamp(tableName string, tunName string, inet4Port netip.Addr, inet6Port netip.Addr, mtu int) error {
	if bridgeUseIptables() {
		return setupBridgeClampIptables(tableName, tunName, inet4Port, inet6Port, mtu)
	}
	return setupBridgeClampRules(tableName, tunName, inet4Port, inet6Port, mtu)
}

func cleanupBridgeNetfilter(tableName string) {
	if bridgeUseIptables() {
		cleanupBridgeIptables(tableName)
		return
	}
	cleanupBridgeNftables(tableName)
}

// Bit 30 stays clear of Android netd's fwmark, which occupies bits 0-20 (netid,
// explicit, protected, permission, uid billing).
const bridgeIptablesMark = "0x40000000/0x40000000"

// The libsu root process inherits a PATH without /system/bin.
func iptablesPath(binary string) string {
	path, err := exec.LookPath(binary)
	if err == nil {
		return path
	}
	if runtime.GOOS == "android" {
		return "/system/bin/" + binary
	}
	return binary
}

func runIptables(binary string, args ...string) error {
	output, err := exec.Command(iptablesPath(binary), args...).CombinedOutput()
	if err != nil {
		return E.Cause(err, binary, " ", strings.Join(args, " "), ": ", strings.TrimSpace(string(output)))
	}
	return nil
}

// iptables refuses input interface matches in nat POSTROUTING, so the mangle
// FORWARD chain marks packets entering from the bridge tun and the nat chain
// masquerades by mark.
func setupBridgeIptables(logger logger.ContextLogger, tableName string, tunName string, inet6 bool) (bool, error) {
	cleanupBridgeIptables(tableName)
	err := setupBridgeIptablesFamily("iptables", tableName, tunName)
	if err != nil {
		cleanupBridgeIptablesFamily("iptables", tableName)
		return false, err
	}
	if !inet6 {
		return false, nil
	}
	err = setupBridgeIptablesFamily("ip6tables", tableName, tunName)
	if err != nil {
		cleanupBridgeIptablesFamily("ip6tables", tableName)
		logger.Debug(E.Cause(err, "IPv6 NAT unavailable, disabling IPv6 forwarding"))
		return false, nil
	}
	return true, nil
}

func setupBridgeIptablesFamily(binary string, tableName string, tunName string) error {
	err := runIptables(binary, "-t", "nat", "-N", tableName)
	if err != nil {
		return err
	}
	err = runIptables(binary, "-t", "nat", "-A", tableName, "-m", "mark", "--mark", bridgeIptablesMark, "-j", "MASQUERADE")
	if err != nil {
		return err
	}
	err = runIptables(binary, "-t", "nat", "-I", "POSTROUTING", "-j", tableName)
	if err != nil {
		return err
	}
	err = runIptables(binary, "-t", "mangle", "-N", tableName)
	if err != nil {
		return err
	}
	err = runIptables(binary, "-t", "mangle", "-A", tableName, "-i", tunName, "-j", "MARK", "--set-xmark", bridgeIptablesMark)
	if err != nil {
		return err
	}
	err = runIptables(binary, "-t", "mangle", "-I", "FORWARD", "-j", tableName)
	if err != nil {
		return err
	}
	return setupBridgeFilterAcceptFamily(binary, tableName, tunName)
}

// netd installs an unconditional DROP in the filter FORWARD chain
// (tetherctrl_FORWARD).
func setupBridgeFilterAcceptFamily(binary string, tableName string, tunName string) error {
	err := runIptables(binary, "-t", "filter", "-N", tableName)
	if err != nil {
		return err
	}
	err = runIptables(binary, "-t", "filter", "-A", tableName, "-i", tunName, "-j", "ACCEPT")
	if err != nil {
		return err
	}
	err = runIptables(binary, "-t", "filter", "-A", tableName, "-o", tunName, "-j", "ACCEPT")
	if err != nil {
		return err
	}
	return runIptables(binary, "-t", "filter", "-I", "FORWARD", "-j", tableName)
}

func setupBridgeClampIptables(tableName string, tunName string, inet4Port netip.Addr, inet6Port netip.Addr, mtu int) error {
	families := []struct {
		binary     string
		port       netip.Addr
		headerSize int
	}{
		{"iptables", inet4Port, 40},
		{"ip6tables", inet6Port, 60},
	}
	for _, family := range families {
		if !family.port.IsValid() {
			continue
		}
		err := runIptables(family.binary, "-t", "mangle", "-F", tableName)
		if err != nil {
			return err
		}
		err = runIptables(family.binary, "-t", "mangle", "-A", tableName, "-i", tunName, "-j", "MARK", "--set-xmark", bridgeIptablesMark)
		if err != nil {
			return err
		}
		err = runIptables(family.binary, "-t", "mangle", "-A", tableName, "-i", tunName,
			"-p", "tcp", "--tcp-flags", "SYN,RST", "SYN",
			"-j", "TCPMSS", "--set-mss", strconv.Itoa(mtu-family.headerSize))
		if err != nil {
			return err
		}
	}
	return nil
}

func cleanupBridgeIptables(tableName string) {
	cleanupBridgeIptablesFamily("iptables", tableName)
	cleanupBridgeIptablesFamily("ip6tables", tableName)
}

func cleanupBridgeIptablesFamily(binary string, tableName string) {
	cleanupBridgeIptablesTable(binary, "nat", "POSTROUTING", tableName)
	cleanupBridgeIptablesTable(binary, "mangle", "FORWARD", tableName)
	cleanupBridgeIptablesTable(binary, "filter", "FORWARD", tableName)
}

func cleanupBridgeIptablesTable(binary string, table string, hookChain string, tableName string) {
	path := iptablesPath(binary)
	_ = exec.Command(path, "-t", table, "-D", hookChain, "-j", tableName).Run()
	_ = exec.Command(path, "-t", table, "-F", tableName).Run()
	_ = exec.Command(path, "-t", table, "-X", tableName).Run()
}

func setupBridgeFamily(tunName string, ruleIndex int, routeTable int, family int, port netip.Addr) error {
	if !port.IsValid() {
		return nil
	}
	link, err := netlink.LinkByName(tunName)
	if err != nil {
		return err
	}
	err = netlink.RouteReplace(bridgeFamilyRoute(link.Attrs().Index, family, port))
	if err != nil {
		return E.Cause(err, "add route")
	}
	for _, rule := range bridgeFamilyRules(tunName, ruleIndex, routeTable, family, port) {
		_ = netlink.RuleDel(rule)
		err = netlink.RuleAdd(rule)
		if err != nil {
			return E.Cause(err, "add rule")
		}
	}
	return nil
}

func removeBridgeFamily(tunName string, ruleIndex int, routeTable int, family int, port netip.Addr) {
	if !port.IsValid() {
		return
	}
	link, err := netlink.LinkByName(tunName)
	if err == nil {
		_ = netlink.RouteDel(bridgeFamilyRoute(link.Attrs().Index, family, port))
	}
	for _, rule := range bridgeFamilyRules(tunName, ruleIndex, routeTable, family, port) {
		_ = netlink.RuleDel(rule)
	}
}

func bridgeFamilyRoute(linkIndex int, family int, port netip.Addr) *netlink.Route {
	bits := port.BitLen()
	route := &netlink.Route{
		LinkIndex: linkIndex,
		Dst:       &net.IPNet{IP: port.AsSlice(), Mask: net.CIDRMask(bits, bits)},
		Table:     unix.RT_TABLE_MAIN,
	}
	if family == unix.AF_INET {
		route.Scope = netlink.Scope(unix.RT_SCOPE_LINK)
	}
	return route
}

func bridgeFamilyRules(tunName string, ruleIndex int, routeTable int, family int, port netip.Addr) []*netlink.Rule {
	forwardTable := unix.RT_TABLE_MAIN
	if routeTable != 0 {
		forwardTable = routeTable
	}

	iifRule := netlink.NewRule()
	iifRule.Priority = ruleIndex
	iifRule.IifName = tunName
	iifRule.Table = forwardTable
	iifRule.Family = family

	toRule := netlink.NewRule()
	toRule.Priority = ruleIndex + 1
	toRule.Dst = netip.PrefixFrom(port, port.BitLen())
	toRule.Table = unix.RT_TABLE_MAIN
	toRule.Family = family

	return []*netlink.Rule{iifRule, toRule}
}

func flushBridgeRouteTable(routeTable int) {
	for _, family := range []int{unix.AF_INET, unix.AF_INET6} {
		routes, err := netlink.RouteListFiltered(family, &netlink.Route{Table: routeTable}, netlink.RT_FILTER_TABLE)
		if err != nil {
			continue
		}
		for _, route := range routes {
			toDelete := route
			_ = netlink.RouteDel(&toDelete)
		}
	}
}

func blackholeBridgeDefault(routeTable int, family int) {
	_ = netlink.RouteReplace(&netlink.Route{
		Table:  routeTable,
		Family: family,
		Type:   unix.RTN_BLACKHOLE,
		Dst:    defaultDestination(family),
	})
}

func activeBridgeFamilies(inet6Port netip.Addr) []int {
	families := []int{unix.AF_INET}
	if inet6Port.IsValid() {
		families = append(families, unix.AF_INET6)
	}
	return families
}

func probeAddress(family int) net.IP {
	if family == unix.AF_INET6 {
		return net.ParseIP("2000::")
	}
	return net.IPv4(1, 1, 1, 1)
}

func defaultDestination(family int) *net.IPNet {
	if family == unix.AF_INET6 {
		return &net.IPNet{IP: net.IPv6zero, Mask: net.CIDRMask(0, 128)}
	}
	return &net.IPNet{IP: net.IPv4zero, Mask: net.CIDRMask(0, 32)}
}

func setupBridgeNftables(tableName string, tunName string) error {
	cleanupBridgeNftables(tableName)
	nft, err := nftables.New()
	if err != nil {
		return err
	}
	table := nft.AddTable(&nftables.Table{
		Family: nftables.TableFamilyINet,
		Name:   tableName,
	})
	chain := nft.AddChain(&nftables.Chain{
		Name:     "postrouting",
		Table:    table,
		Type:     nftables.ChainTypeNAT,
		Hooknum:  nftables.ChainHookPostrouting,
		Priority: nftables.ChainPriorityNATSource,
	})
	// The nft_fullcone verb, like masquerade, sources from the routing-chosen egress
	// interface.
	var sourceNat expr.Any = &expr.Masq{}
	if fullConeSupported() {
		sourceNat = &expr.FullCone{}
	}
	nft.AddRule(&nftables.Rule{
		Table: table,
		Chain: chain,
		Exprs: []expr.Any{
			&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1},
			&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: nftIfname(tunName)},
			sourceNat,
		},
	})
	nft.AddChain(&nftables.Chain{
		Name:     "forward",
		Table:    table,
		Type:     nftables.ChainTypeFilter,
		Hooknum:  nftables.ChainHookForward,
		Priority: nftables.ChainPriorityMangle,
	})
	return nft.Flush()
}

// nft_exthdr writes the MSS option unconditionally — unlike pf's max-mss or
// xt_TCPMSS it would also raise a smaller advertised MSS — so the rule matches
// only when the advertised MSS exceeds the clamp value.
func setupBridgeClampRules(tableName string, tunName string, inet4Port netip.Addr, inet6Port netip.Addr, mtu int) error {
	nft, err := nftables.New()
	if err != nil {
		return err
	}
	table := &nftables.Table{
		Family: nftables.TableFamilyINet,
		Name:   tableName,
	}
	chain := &nftables.Chain{
		Name:  "forward",
		Table: table,
	}
	nft.FlushChain(chain)
	families := []struct {
		protocol   byte
		port       netip.Addr
		headerSize int
	}{
		{unix.NFPROTO_IPV4, inet4Port, 40},
		{unix.NFPROTO_IPV6, inet6Port, 60},
	}
	for _, family := range families {
		if !family.port.IsValid() {
			continue
		}
		clamp := binaryutil.BigEndian.PutUint16(uint16(mtu - family.headerSize))
		nft.AddRule(&nftables.Rule{
			Table: table,
			Chain: chain,
			Exprs: []expr.Any{
				&expr.Meta{Key: expr.MetaKeyNFPROTO, Register: 1},
				&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{family.protocol}},
				&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1},
				&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: nftIfname(tunName)},
				&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1},
				&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{unix.IPPROTO_TCP}},
				&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseTransportHeader, Offset: 13, Len: 1},
				&expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 1, Mask: []byte{0x02}, Xor: []byte{0x00}},
				&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{0x02}},
				&expr.Exthdr{DestRegister: 1, Type: 2, Offset: 2, Len: 2, Op: expr.ExthdrOpTcpopt},
				&expr.Cmp{Op: expr.CmpOpGt, Register: 1, Data: clamp},
				&expr.Immediate{Register: 1, Data: clamp},
				&expr.Exthdr{SourceRegister: 1, Type: 2, Offset: 2, Len: 2, Op: expr.ExthdrOpTcpopt},
			},
		})
	}
	return nft.Flush()
}

func cleanupBridgeNftables(tableName string) {
	nft, err := nftables.New()
	if err != nil {
		return
	}
	table, err := nft.ListTableOfFamily(tableName, nftables.TableFamilyINet)
	if err != nil || table == nil {
		return
	}
	nft.DelTable(table)
	_ = nft.Flush()
}

func fullConeSupported() bool {
	if runtime.GOOS == "android" {
		return false
	}
	if bridgeUseIptables() {
		return false
	}
	fullConeProbeOnce.Do(func() {
		fullConeProbeResult = probeFullCone()
	})
	return fullConeProbeResult
}

const fullConeProbeTable = "sing-box-fullcone-probe"

// The kernel loads and validates the expression's module when the batch commits:
// a clean flush means the verb is available, a rejected one rolls back atomically.
func probeFullCone() bool {
	deleteFullConeProbe()
	nft, err := nftables.New()
	if err != nil {
		return false
	}
	table := nft.AddTable(&nftables.Table{
		Family: nftables.TableFamilyINet,
		Name:   fullConeProbeTable,
	})
	chain := nft.AddChain(&nftables.Chain{
		Name:     "postrouting",
		Table:    table,
		Type:     nftables.ChainTypeNAT,
		Hooknum:  nftables.ChainHookPostrouting,
		Priority: nftables.ChainPriorityNATSource,
	})
	nft.AddRule(&nftables.Rule{
		Table: table,
		Chain: chain,
		Exprs: []expr.Any{
			&expr.Meta{Key: expr.MetaKeyOIFNAME, Register: 1},
			&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: nftIfname("sing-box-probe0")},
			&expr.FullCone{},
		},
	})
	supported := nft.Flush() == nil
	deleteFullConeProbe()
	return supported
}

func deleteFullConeProbe() {
	nft, err := nftables.New()
	if err != nil {
		return
	}
	table, err := nft.ListTableOfFamily(fullConeProbeTable, nftables.TableFamilyINet)
	if err != nil || table == nil {
		return
	}
	nft.DelTable(table)
	_ = nft.Flush()
}

func nftIfname(name string) []byte {
	padded := make([]byte, 16)
	copy(padded, name)
	return padded
}
