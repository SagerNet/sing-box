package inbound

import (
	"context"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/redir"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/x/list"
)

const (
	tableNameOutput      = "sing-box-output"
	tableNameForward     = "sing-box-forward"
	tableNamePreRouteing = "sing-box-prerouting"
)

type tunAutoRedirect struct {
	myInboundAdapter
	tunOptions      *tun.Options
	interfaceFinder control.InterfaceFinder
	networkMonitor  tun.NetworkUpdateMonitor
	networkCallback *list.Element[tun.NetworkUpdateCallback]
	enableIPv4      bool
	enableIPv6      bool
	localAddresses4 []netip.Prefix
	localAddresses6 []netip.Prefix
	iptablesPath    string
	ip6tablesPath   string
	androidSu       bool
	suPath          string
}

func newAutoRedirect(t *Tun) (*tunAutoRedirect, error) {
	if !C.IsLinux {
		return nil, E.New("only supported on linux")
	}
	server := &tunAutoRedirect{
		myInboundAdapter: myInboundAdapter{
			protocol: C.TypeRedirect,
			network:  []string{N.NetworkTCP},
			ctx:      t.ctx,
			router:   t.router,
			logger:   t.logger,
			tag:      t.tag,
		},
		tunOptions:      &t.tunOptions,
		interfaceFinder: t.router.InterfaceFinder(),
		networkMonitor:  t.router.NetworkMonitor(),
	}
	server.connHandler = server
	if len(t.tunOptions.Inet4Address) > 0 {
		server.enableIPv4 = true
		if C.IsAndroid {
			server.iptablesPath = "/system/bin/iptables"
			userId := os.Getuid()
			if userId != 0 {
				var (
					suPath string
					err    error
				)
				if t.platformInterface != nil {
					suPath, err = exec.LookPath("/bin/su")
				} else {
					suPath, err = exec.LookPath("su")
				}
				if err == nil {
					server.androidSu = true
					server.suPath = suPath
				} else {
					return nil, E.Extend(E.Cause(err, "root permission is required for auto redirect"), os.Getenv("PATH"))
				}
			}
		} else {
			iptablesPath, err := exec.LookPath("iptables")
			if err != nil {
				return nil, E.Cause(err, "iptables is required")
			}
			server.iptablesPath = iptablesPath
		}
	}
	if !C.IsAndroid && len(t.tunOptions.Inet6Address) > 0 {
		err := server.initializeIP6Tables()
		if err != nil {
			t.logger.Debug("device has no ip6tables nat support: ", err)
		}
	}
	var listenAddr netip.Addr
	if C.IsAndroid {
		listenAddr = netip.AddrFrom4([4]byte{127, 0, 0, 1})
	} else if server.enableIPv6 {
		listenAddr = netip.IPv6Unspecified()
	} else {
		listenAddr = netip.IPv4Unspecified()
	}
	server.listenOptions.Listen = option.NewListenAddress(listenAddr)
	return server, nil
}

func (t *tunAutoRedirect) initializeIP6Tables() error {
	ip6tablesPath, err := exec.LookPath("ip6tables")
	if err != nil {
		return err
	}
	/*output, err := exec.Command(ip6tablesPath, "-t nat -L", tableNameOutput).CombinedOutput()
	switch exitErr := err.(type) {
	case nil:
	case *exec.ExitError:
		if exitErr.ExitCode() != 1 {
			return E.Extend(err, string(output))
		}
	default:
		return err
	}*/
	t.ip6tablesPath = ip6tablesPath
	t.enableIPv6 = true
	return nil
}

func (t *tunAutoRedirect) Start(tunName string) error {
	err := t.myInboundAdapter.Start()
	if err != nil {
		return E.Cause(err, "start redirect server")
	}
	if t.enableIPv4 {
		t.cleanupIPTables(t.iptablesPath)
	}
	if t.enableIPv6 {
		t.cleanupIPTables(t.ip6tablesPath)
	}
	err = t.updateInterfaces(false)
	if err != nil {
		return err
	}
	if t.enableIPv4 {
		err = t.setupIPTables(t.iptablesPath, tunName)
		if err != nil {
			return err
		}
	}
	if t.enableIPv6 {
		err = t.setupIPTables(t.ip6tablesPath, tunName)
		if err != nil {
			return err
		}
	}
	t.networkCallback = t.networkMonitor.RegisterCallback(func() {
		rErr := t.updateInterfaces(true)
		if rErr != nil {
			t.logger.Error("recreate prerouting rules: ", rErr)
		}
	})
	return nil
}

func (t *tunAutoRedirect) updateInterfaces(recreate bool) error {
	addresses := common.Filter(common.FlatMap(common.Filter(t.interfaceFinder.Interfaces(), func(it control.Interface) bool {
		return it.Name != t.tunOptions.Name
	}), func(it control.Interface) []netip.Prefix {
		return it.Addresses
	}), func(it netip.Prefix) bool {
		address := it.Addr()
		return !(address.IsLoopback() || address.IsLinkLocalUnicast())
	})
	oldLocalAddresses4 := t.localAddresses4
	oldLocalAddresses6 := t.localAddresses6
	localAddresses4 := common.Filter(addresses, func(it netip.Prefix) bool { return it.Addr().Is4() })
	localAddresses6 := common.Filter(addresses, func(it netip.Prefix) bool { return it.Addr().Is6() })
	t.localAddresses4 = localAddresses4
	t.localAddresses6 = localAddresses6
	if !recreate || t.androidSu {
		return nil
	}
	if t.enableIPv4 {
		if !slices.Equal(localAddresses4, oldLocalAddresses4) {
			err := t.setupIPTablesPreRouting(t.iptablesPath, true)
			if err != nil {
				return err
			}
		}
	}
	if t.enableIPv6 {
		if !slices.Equal(localAddresses6, oldLocalAddresses6) {
			err := t.setupIPTablesPreRouting(t.ip6tablesPath, true)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *tunAutoRedirect) Close() error {
	t.networkMonitor.UnregisterCallback(t.networkCallback)
	if t.enableIPv4 {
		t.cleanupIPTables(t.iptablesPath)
	}
	if t.enableIPv6 {
		t.cleanupIPTables(t.ip6tablesPath)
	}
	return t.myInboundAdapter.Close()
}

func (t *tunAutoRedirect) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	destination, err := redir.GetOriginalDestination(conn)
	if err != nil {
		return E.Cause(err, "get redirect destination")
	}
	metadata.Destination = M.SocksaddrFromNetIP(destination)
	return t.newConnection(ctx, conn, metadata)
}

func (t *tunAutoRedirect) setupIPTables(iptablesPath string, tunName string) error {
	// OUTPUT
	err := t.runShell(iptablesPath, "-t nat -N", tableNameOutput)
	if err != nil {
		return err
	}
	err = t.runShell(iptablesPath, "-t nat -A", tableNameOutput,
		"-p tcp -o", tunName,
		"-j REDIRECT --to-ports", M.AddrPortFromNet(t.tcpListener.Addr()).Port())
	if err != nil {
		return err
	}
	err = t.runShell(iptablesPath, "-t nat -I OUTPUT -j", tableNameOutput)
	if err != nil {
		return err
	}
	if !t.androidSu {
		// FORWARD
		err = t.runShell(iptablesPath, "-N", tableNameForward)
		if err != nil {
			return err
		}
		err = t.runShell(iptablesPath, "-A", tableNameForward,
			"-i", tunName, "-j", "ACCEPT")
		if err != nil {
			return err
		}
		err = t.runShell(iptablesPath, "-A", tableNameForward,
			"-o", tunName, "-j", "ACCEPT")
		if err != nil {
			return err
		}
		err = t.runShell(iptablesPath, "-I FORWARD -j", tableNameForward)
		if err != nil {
			return err
		}
		// PREROUTING
		err = t.setupIPTablesPreRouting(iptablesPath, false)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *tunAutoRedirect) setupIPTablesPreRouting(iptablesPath string, recreate bool) error {
	var err error
	if !recreate {
		err = t.runShell(iptablesPath, "-t nat -N", tableNamePreRouteing)
	} else {
		err = t.runShell(iptablesPath, "-t nat -F", tableNamePreRouteing)
	}
	if err != nil {
		return err
	}
	var (
		routeAddress        []netip.Prefix
		routeExcludeAddress []netip.Prefix
	)
	if t.iptablesPath == iptablesPath {
		routeAddress = t.tunOptions.Inet4RouteAddress
		routeExcludeAddress = t.tunOptions.Inet4RouteExcludeAddress
	} else {
		routeAddress = t.tunOptions.Inet6RouteAddress
		routeExcludeAddress = t.tunOptions.Inet6RouteExcludeAddress
	}
	if len(routeAddress) > 0 && (len(t.tunOptions.IncludeInterface) > 0 || len(t.tunOptions.IncludeUID) > 0) {
		return E.New("`*_route_address` is conflict with `include_interface` or `include_uid`")
	}
	if len(routeExcludeAddress) > 0 {
		for _, address := range routeExcludeAddress {
			err = t.runShell(iptablesPath, "-t nat -A", tableNamePreRouteing,
				"-d", address.String(), "-j RETURN")
			if err != nil {
				return err
			}
		}
	}
	if len(t.tunOptions.ExcludeInterface) > 0 {
		for _, name := range t.tunOptions.ExcludeInterface {
			err = t.runShell(iptablesPath, "-t nat -A", tableNamePreRouteing,
				"-i", name, "-j RETURN")
			if err != nil {
				return err
			}
		}
	}
	if len(t.tunOptions.ExcludeUID) > 0 {
		for _, uid := range t.tunOptions.ExcludeUID {
			err = t.runShell(iptablesPath, "-t nat -A", tableNamePreRouteing,
				"-m owner --uid-owner", uid, "-j RETURN")
			if err != nil {
				return err
			}
		}
	}
	var addresses []netip.Prefix
	if t.iptablesPath == iptablesPath {
		addresses = t.localAddresses4
	} else {
		addresses = t.localAddresses6
	}
	for _, address := range addresses {
		err = t.runShell(iptablesPath, "-t nat -A", tableNamePreRouteing, "-d", address.String(), "-j RETURN")
		if err != nil {
			return err
		}
	}
	if len(routeAddress) > 0 {
		for _, address := range routeAddress {
			err = t.runShell(iptablesPath, "-t nat -A", tableNamePreRouteing,
				"-d", address.String(), "-p tcp -j REDIRECT --to-ports", M.AddrPortFromNet(t.tcpListener.Addr()).Port())
			if err != nil {
				return err
			}
		}
	} else if len(t.tunOptions.IncludeInterface) > 0 || len(t.tunOptions.IncludeUID) > 0 {
		for _, name := range t.tunOptions.IncludeInterface {
			err = t.runShell(iptablesPath, "-t nat -A", tableNamePreRouteing,
				"-i", name, "-p tcp -j REDIRECT --to-ports", M.AddrPortFromNet(t.tcpListener.Addr()).Port())
			if err != nil {
				return err
			}
		}
		for _, uidRange := range t.tunOptions.IncludeUID {
			for i := uidRange.Start; i <= uidRange.End; i++ {
				err = t.runShell(iptablesPath, "-t nat -A", tableNamePreRouteing,
					"-m owner --uid-owner", i, "-p tcp -j REDIRECT --to-ports", M.AddrPortFromNet(t.tcpListener.Addr()).Port())
				if err != nil {
					return err
				}
			}
		}
	} else {
		err = t.runShell(iptablesPath, "-t nat -A", tableNamePreRouteing,
			"-p tcp -j REDIRECT --to-ports", M.AddrPortFromNet(t.tcpListener.Addr()).Port())
		if err != nil {
			return err
		}
	}
	err = t.runShell(iptablesPath, "-t nat -I PREROUTING -j", tableNamePreRouteing)
	if err != nil {
		return err
	}
	return nil
}

func (t *tunAutoRedirect) cleanupIPTables(iptablesPath string) {
	_ = t.runShell(iptablesPath, "-t nat -D OUTPUT -j", tableNameOutput)
	_ = t.runShell(iptablesPath, "-t nat -F", tableNameOutput)
	_ = t.runShell(iptablesPath, "-t nat -X", tableNameOutput)
	if !t.androidSu {
		_ = t.runShell(iptablesPath, "-D FORWARD -j", tableNameForward)
		_ = t.runShell(iptablesPath, "-F", tableNameForward)
		_ = t.runShell(iptablesPath, "-X", tableNameForward)
		_ = t.runShell(iptablesPath, "-t nat -D PREROUTING -j", tableNamePreRouteing)
		_ = t.runShell(iptablesPath, "-t nat -F", tableNamePreRouteing)
		_ = t.runShell(iptablesPath, "-t nat -X", tableNamePreRouteing)
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
