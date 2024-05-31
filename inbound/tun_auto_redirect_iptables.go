//go:build linux

package inbound

import (
	"net/netip"
	"os/exec"
	"strings"

	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"

	"golang.org/x/sys/unix"
)

const (
	iptablesTableNameOutput      = "sing-box-output"
	iptablesTableNameForward     = "sing-box-forward"
	iptablesTableNamePreRouteing = "sing-box-prerouting"
)

func (t *tunAutoRedirect) iptablesPathForFamily(family int) string {
	if family == unix.AF_INET {
		return t.iptablesPath
	} else {
		return t.ip6tablesPath
	}
}

func (t *tunAutoRedirect) setupIPTables(family int) error {
	iptablesPath := t.iptablesPathForFamily(family)
	// OUTPUT
	err := t.runShell(iptablesPath, "-t nat -N", iptablesTableNameOutput)
	if err != nil {
		return err
	}
	err = t.runShell(iptablesPath, "-t nat -A", iptablesTableNameOutput,
		"-p tcp -o", t.tunOptions.Name,
		"-j REDIRECT --to-ports", M.AddrPortFromNet(t.tcpListener.Addr()).Port())
	if err != nil {
		return err
	}
	err = t.runShell(iptablesPath, "-t nat -I OUTPUT -j", iptablesTableNameOutput)
	if err != nil {
		return err
	}
	if !t.androidSu {
		// FORWARD
		err = t.runShell(iptablesPath, "-N", iptablesTableNameForward)
		if err != nil {
			return err
		}
		err = t.runShell(iptablesPath, "-A", iptablesTableNameForward,
			"-i", t.tunOptions.Name, "-j", "ACCEPT")
		if err != nil {
			return err
		}
		err = t.runShell(iptablesPath, "-A", iptablesTableNameForward,
			"-o", t.tunOptions.Name, "-j", "ACCEPT")
		if err != nil {
			return err
		}
		err = t.runShell(iptablesPath, "-I FORWARD -j", iptablesTableNameForward)
		if err != nil {
			return err
		}
		// PREROUTING
		err = t.setupIPTablesPreRouting(family)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *tunAutoRedirect) setupIPTablesPreRouting(family int) error {
	iptablesPath := t.iptablesPathForFamily(family)
	err := t.runShell(iptablesPath, "-t nat -N", iptablesTableNamePreRouteing)
	if err != nil {
		return err
	}
	var (
		routeAddress        []netip.Prefix
		routeExcludeAddress []netip.Prefix
	)
	if family == unix.AF_INET {
		routeAddress = t.tunOptions.Inet4RouteAddress
		routeExcludeAddress = t.tunOptions.Inet4RouteExcludeAddress
	} else {
		routeAddress = t.tunOptions.Inet6RouteAddress
		routeExcludeAddress = t.tunOptions.Inet6RouteExcludeAddress
	}
	if len(routeAddress) > 0 && (len(t.tunOptions.IncludeInterface) > 0 || len(t.tunOptions.IncludeUID) > 0) {
		return E.New("`*_route_address` is conflict with `include_interface` or `include_uid`")
	}
	err = t.runShell(iptablesPath, "-t nat -A", iptablesTableNamePreRouteing,
		"-i", t.tunOptions.Name, "-j RETURN")
	if err != nil {
		return err
	}
	for _, address := range routeExcludeAddress {
		err = t.runShell(iptablesPath, "-t nat -A", iptablesTableNamePreRouteing,
			"-d", address.String(), "-j RETURN")
		if err != nil {
			return err
		}
	}
	for _, name := range t.tunOptions.ExcludeInterface {
		err = t.runShell(iptablesPath, "-t nat -A", iptablesTableNamePreRouteing,
			"-i", name, "-j RETURN")
		if err != nil {
			return err
		}
	}
	for _, uid := range t.tunOptions.ExcludeUID {
		err = t.runShell(iptablesPath, "-t nat -A", iptablesTableNamePreRouteing,
			"-m owner --uid-owner", uid, "-j RETURN")
		if err != nil {
			return err
		}
	}
	var dnsServerAddress netip.Addr
	if family == unix.AF_INET {
		dnsServerAddress = t.tunOptions.Inet4Address[0].Addr().Next()
	} else {
		dnsServerAddress = t.tunOptions.Inet6Address[0].Addr().Next()
	}
	if len(routeAddress) > 0 {
		for _, address := range routeAddress {
			err = t.runShell(iptablesPath, "-t nat -A", iptablesTableNamePreRouteing,
				"-d", address.String(), "-p udp --dport 53 -j DNAT --to", dnsServerAddress)
			if err != nil {
				return err
			}
		}
	} else if len(t.tunOptions.IncludeInterface) > 0 || len(t.tunOptions.IncludeUID) > 0 {
		for _, name := range t.tunOptions.IncludeInterface {
			err = t.runShell(iptablesPath, "-t nat -A", iptablesTableNamePreRouteing,
				"-i", name, "-p udp --dport 53 -j DNAT --to", dnsServerAddress)
			if err != nil {
				return err
			}
		}
		for _, uidRange := range t.tunOptions.IncludeUID {
			for uid := uidRange.Start; uid <= uidRange.End; uid++ {
				err = t.runShell(iptablesPath, "-t nat -A", iptablesTableNamePreRouteing,
					"-m owner --uid-owner", uid, "-p udp --dport 53 -j DNAT --to", dnsServerAddress)
				if err != nil {
					return err
				}
			}
		}
	} else {
		err = t.runShell(iptablesPath, "-t nat -A", iptablesTableNamePreRouteing,
			"-p udp --dport 53 -j DNAT --to", dnsServerAddress)
		if err != nil {
			return err
		}
	}

	err = t.runShell(iptablesPath, "-t nat -A", iptablesTableNamePreRouteing, "-m addrtype --dst-type LOCAL -j RETURN")
	if err != nil {
		return err
	}

	if len(routeAddress) > 0 {
		for _, address := range routeAddress {
			err = t.runShell(iptablesPath, "-t nat -A", iptablesTableNamePreRouteing,
				"-d", address.String(), "-p tcp -j REDIRECT --to-ports", M.AddrPortFromNet(t.tcpListener.Addr()).Port())
			if err != nil {
				return err
			}
		}
	} else if len(t.tunOptions.IncludeInterface) > 0 || len(t.tunOptions.IncludeUID) > 0 {
		for _, name := range t.tunOptions.IncludeInterface {
			err = t.runShell(iptablesPath, "-t nat -A", iptablesTableNamePreRouteing,
				"-i", name, "-p tcp -j REDIRECT --to-ports", M.AddrPortFromNet(t.tcpListener.Addr()).Port())
			if err != nil {
				return err
			}
		}
		for _, uidRange := range t.tunOptions.IncludeUID {
			for uid := uidRange.Start; uid <= uidRange.End; uid++ {
				err = t.runShell(iptablesPath, "-t nat -A", iptablesTableNamePreRouteing,
					"-m owner --uid-owner", uid, "-p tcp -j REDIRECT --to-ports", M.AddrPortFromNet(t.tcpListener.Addr()).Port())
				if err != nil {
					return err
				}
			}
		}
	} else {
		err = t.runShell(iptablesPath, "-t nat -A", iptablesTableNamePreRouteing,
			"-p tcp -j REDIRECT --to-ports", M.AddrPortFromNet(t.tcpListener.Addr()).Port())
		if err != nil {
			return err
		}
	}
	err = t.runShell(iptablesPath, "-t nat -I PREROUTING -j", iptablesTableNamePreRouteing)
	if err != nil {
		return err
	}
	return nil
}

func (t *tunAutoRedirect) cleanupIPTables(family int) {
	iptablesPath := t.iptablesPathForFamily(family)
	_ = t.runShell(iptablesPath, "-t nat -D OUTPUT -j", iptablesTableNameOutput)
	_ = t.runShell(iptablesPath, "-t nat -F", iptablesTableNameOutput)
	_ = t.runShell(iptablesPath, "-t nat -X", iptablesTableNameOutput)
	if !t.androidSu {
		_ = t.runShell(iptablesPath, "-D FORWARD -j", iptablesTableNameForward)
		_ = t.runShell(iptablesPath, "-F", iptablesTableNameForward)
		_ = t.runShell(iptablesPath, "-X", iptablesTableNameForward)
		_ = t.runShell(iptablesPath, "-t nat -D PREROUTING -j", iptablesTableNamePreRouteing)
		_ = t.runShell(iptablesPath, "-t nat -F", iptablesTableNamePreRouteing)
		_ = t.runShell(iptablesPath, "-t nat -X", iptablesTableNamePreRouteing)
	}
}

func (t *tunAutoRedirect) runShell(commands ...any) error {
	commandStr := strings.Join(F.MapToString(commands), " ")
	var command *exec.Cmd
	if t.androidSu {
		command = exec.Command(t.suPath, "-c", commandStr)
	} else {
		commandArray := strings.Split(commandStr, " ")
		command = exec.Command(commandArray[0], commandArray[1:]...)
	}
	combinedOutput, err := command.CombinedOutput()
	if err != nil {
		return E.Extend(err, F.ToString(commandStr, ": ", string(combinedOutput)))
	}
	return nil
}
