package inbound

import (
	"context"
	"net"
	"net/netip"
	"os/exec"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/redir"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

const (
	tableNameOutput      = "sing-box-output"
	tableNamePreRouteing = "sing-box-prerouting"
	tableNameForward     = "sing-box-forward"
)

type tunAutoRedirect struct {
	myInboundAdapter
	iptablesPath  string
	androidSu     bool
	enableIPv6    bool
	ip6tablesPath string
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
			listenOptions: option.ListenOptions{
				Listen: option.NewListenAddress(netip.AddrFrom4([4]byte{127, 0, 0, 1})),
			},
		},
	}
	server.connHandler = server
	if C.IsAndroid && t.platformInterface != nil {
		server.iptablesPath = "/system/bin/iptables"
	} else {
		iptablesPath, err := exec.LookPath("iptables")
		if err != nil {
			return nil, E.Cause(err, "iptables is required")
		}
		server.iptablesPath = iptablesPath
	}
	if !C.IsAndroid && len(t.tunOptions.Inet6Address) > 0 {
		err := server.initializeIP6Tables()
		if err != nil {
			return nil, err
		}
		t.logger.Debug("device has no ip6tables nat support: ", err)
	}
	return server, nil
}

func (t *tunAutoRedirect) initializeIP6Tables() error {
	ip6tablesPath, err := exec.LookPath("ip6tables")
	if err != nil {
		return err
	}
	output, err := exec.Command(ip6tablesPath, "-t nat -L", tableNameOutput).CombinedOutput()
	switch exitErr := err.(type) {
	case nil:
	case *exec.ExitError:
		if exitErr.ExitCode() != 1 {
			return E.Extend(err, string(output))
		}
	default:
		return err
	}
	t.ip6tablesPath = ip6tablesPath
	t.enableIPv6 = true
	return nil
}

func (t *tunAutoRedirect) Start(tunName string) error {
	err := t.myInboundAdapter.Start()
	if err != nil {
		return E.Cause(err, "start redirect server")
	}
	t.cleanupIPTables(t.iptablesPath)
	if t.enableIPv6 {
		t.cleanupIPTables(t.ip6tablesPath)
	}
	err = t.setupIPTables(t.iptablesPath, tunName)
	if err != nil {
		return err
	}
	if t.enableIPv6 {
		err = t.setupIPTables(t.ip6tablesPath, tunName)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *tunAutoRedirect) Close() error {
	t.cleanupIPTables(t.iptablesPath)
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
	_ = t.runShell(iptablesPath, "-t nat -F", tableNameOutput)
	_ = t.runShell(iptablesPath, "-t nat -X", tableNameOutput)
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
	// PREROUTING
	_ = t.runShell(iptablesPath, "-t nat -F", tableNamePreRouteing)
	_ = t.runShell(iptablesPath, "-t nat -X", tableNamePreRouteing)
	err = t.runShell(iptablesPath, "-t nat -N", tableNamePreRouteing)
	if err != nil {
		return err
	}
	// Hijack DNS requests
	err = t.runShell(iptablesPath, "-t nat -A", tableNamePreRouteing,
		"! -i", tunName, "-p tcp --dport 53 -j REDIRECT --to-ports", M.AddrPortFromNet(t.tcpListener.Addr()).Port())
	if err != nil {
		return err
	}
	err = t.runShell(iptablesPath, "-t nat -A", tableNamePreRouteing,
		"! -i", tunName, "-p udp --dport 53 -j REDIRECT --to-ports", M.AddrPortFromNet(t.tcpListener.Addr()).Port())
	if err != nil {
		return err
	}
	for _, netIf := range t.router.(adapter.Router).InterfaceFinder().Interfaces() {
		for _, addr := range netIf.Addresses {
			if !addr.Addr().Is4() {
				continue
			}
			err = t.runShell(iptablesPath, "-t nat -A", tableNamePreRouteing, "-d", addr.String(), "-j RETURN")
			if err != nil {
				return err
			}
		}
	}
	err = t.runShell(iptablesPath, "-t nat -A", tableNamePreRouteing,
		"-p tcp -j REDIRECT --to-ports", M.AddrPortFromNet(t.tcpListener.Addr()).Port())
	if err != nil {
		return err
	}
	err = t.runShell(iptablesPath, "-t nat -I PREROUTING -j", tableNamePreRouteing)
	if err != nil {
		return err
	}
	// FORWARD
	_ = t.runShell(iptablesPath, "-t nat -F", tableNameForward)
	_ = t.runShell(iptablesPath, "-t nat -X", tableNameForward)
	err = t.runShell(iptablesPath, "-t nat -N", tableNameForward)
	if err != nil {
		return err
	}
	err = t.runShell(iptablesPath, "-t nat -A", tableNameForward,
		"-i", tunName, "-j", "ACCEPT")
	if err != nil {
		return err
	}
	err = t.runShell(iptablesPath, "-t nat -A", tableNameForward,
		"-o", tunName, "-j", "ACCEPT")
	if err != nil {
		return err
	}
	err = t.runShell(iptablesPath, "-t nat -I FORWARD -j", tableNameForward)
	if err != nil {
		return err
	}
	return nil
}

func (t *tunAutoRedirect) cleanupIPTables(iptablesPath string) {
	_ = t.runShell(iptablesPath, "-t nat -D OUTPUT -j", tableNameOutput)
	_ = t.runShell(iptablesPath, "-t nat -D PREROUTING -j", tableNamePreRouteing)
	_ = t.runShell(iptablesPath, "-t nat -D FORWARD -j", tableNameForward)
}

func (t *tunAutoRedirect) runShell(commands ...any) error {
	commandStr := strings.Join(F.MapToString(commands), " ")
	var command *exec.Cmd
	if t.androidSu {
		command = exec.Command("/bin/su", "-c", commandStr)
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
