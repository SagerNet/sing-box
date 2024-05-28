package inbound

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/redir"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/libbox/platform"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type Redirect struct {
	myInboundAdapter
	platformInterface platform.Interface
	autoRedirect      option.AutoRedirectOptions
	needSu            bool
	suPath            string
}

func NewRedirect(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.RedirectInboundOptions, platformInterface platform.Interface) (*Redirect, error) {
	redirect := &Redirect{
		myInboundAdapter: myInboundAdapter{
			protocol:      C.TypeRedirect,
			network:       []string{N.NetworkTCP},
			ctx:           ctx,
			router:        router,
			logger:        logger,
			tag:           tag,
			listenOptions: options.ListenOptions,
		},
		platformInterface: platformInterface,
		autoRedirect:      common.PtrValueOrDefault(options.AutoRedirect),
	}
	if redirect.autoRedirect.Enabled {
		if !C.IsAndroid {
			return nil, E.New("auto redirect is only supported on Android")
		}
		userId := os.Getuid()
		if userId != 0 {
			suPath, err := exec.LookPath("/bin/su")
			if err == nil {
				redirect.needSu = true
				redirect.suPath = suPath
			} else if redirect.autoRedirect.ContinueOnNoPermission {
				redirect.autoRedirect.Enabled = false
			} else {
				return nil, E.Extend(E.Cause(err, "root permission is required for auto redirect"), os.Getenv("PATH"))
			}
		}
	}
	redirect.connHandler = redirect
	return redirect, nil
}

func (r *Redirect) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	destination, err := redir.GetOriginalDestination(conn)
	if err != nil {
		return E.Cause(err, "get redirect destination")
	}
	metadata.Destination = M.SocksaddrFromNetIP(destination)
	return r.newConnection(ctx, conn, metadata)
}

func (r *Redirect) Start() error {
	err := r.myInboundAdapter.Start()
	if err != nil {
		return err
	}
	if r.autoRedirect.Enabled {
		r.cleanupRedirect()
		err = r.setupRedirect()
		if err != nil {
			var exitError *exec.ExitError
			if errors.As(err, &exitError) && exitError.ExitCode() == 13 && r.autoRedirect.ContinueOnNoPermission {
				r.logger.Error(E.Cause(err, "setup auto redirect"))
				return nil
			}
			r.cleanupRedirect()
			return E.Cause(err, "setup auto redirect")
		}
	}
	return nil
}

func (r *Redirect) Close() error {
	if r.autoRedirect.Enabled {
		r.cleanupRedirect()
	}
	return r.myInboundAdapter.Close()
}

func (r *Redirect) setupRedirect() error {
	tableName := "sing-box"
	rules := `
set -e -o pipefail
iptables -t nat -N sing-box
`
	rules += strings.Join(common.FlatMap(r.router.(adapter.Router).InterfaceFinder().Interfaces(), func(it control.Interface) []string {
		return common.Map(common.Filter(it.Addresses, func(it netip.Prefix) bool { return it.Addr().Is4() }), func(it netip.Prefix) string {
			return "iptables -t nat -A " + tableName + " -p tcp -j RETURN -d " + it.String()
		})
	}), "\n")
	var (
		myUid           = uint32(os.Getuid())
		perAppProxyList []uint32
		perAppProxyMap  = make(map[uint32]bool)
		perAppProxyMode int32
		err             error
	)
	if r.platformInterface != nil {
		perAppProxyMode = r.platformInterface.PerAppProxyMode()
		if perAppProxyMode != platform.PerAppProxyModeDisabled {
			perAppProxyList, err = r.platformInterface.PerAppProxyList()
			if err != nil {
				return E.Cause(err, "read per app proxy configuration")
			}
		}
		for _, proxyUID := range perAppProxyList {
			perAppProxyMap[proxyUID] = true
		}
	}
	excludeUser := func(userID uint32) {
		if perAppProxyMode != platform.PerAppProxyModeInclude {
			perAppProxyMap[userID] = false
		} else {
			delete(perAppProxyMap, userID)
		}
	}
	excludeUser(myUid)
	if myUid != 0 && myUid != 2000 {
		excludeUser(0)
	}
	perAppProxyList = perAppProxyList[:0]
	for uid := range perAppProxyMap {
		perAppProxyList = append(perAppProxyList, uid)
	}
	sort.SliceStable(perAppProxyList, func(i, j int) bool {
		return perAppProxyList[i] < perAppProxyList[j]
	})
	redirectPortStr := F.ToString(M.AddrPortFromNet(r.tcpListener.Addr()).Port())
	if perAppProxyMode != platform.PerAppProxyModeInclude {
		rules += "\n" + strings.Join(common.Map(perAppProxyList, func(it uint32) string {
			return "iptables -t nat -A " + tableName + " -j RETURN -m owner --uid-owner " + F.ToString(it)
		}), "\n")
		rules += "\niptables -t nat -A " + tableName + " -p tcp -j REDIRECT --to-ports " + redirectPortStr
	} else {
		rules += "\n" + strings.Join(common.Map(perAppProxyList, func(it uint32) string {
			return "iptables -t nat -A " + tableName + " -p tcp -j REDIRECT --to-ports " + redirectPortStr + " -m owner --uid-owner " + F.ToString(it)
		}), "\n")
	}
	rules += "\niptables -t nat -A OUTPUT -p tcp -j " + tableName
	for _, ruleLine := range strings.Split(rules, "\n") {
		ruleLine = strings.TrimSpace(ruleLine)
		if ruleLine == "" {
			continue
		}
		r.logger.Debug("# ", ruleLine)
	}
	return r.runAndroidShell(rules)
}

func (r *Redirect) cleanupRedirect() {
	_ = r.runAndroidShell(`
iptables -t nat -D OUTPUT -p tcp -j sing-box
iptables -t nat -F sing-box
iptables -t nat -X sing-box
`)
}

func (r *Redirect) runAndroidShell(content string) error {
	var command *exec.Cmd
	if r.needSu {
		command = exec.Command(r.suPath, "-c", "sh")
	} else {
		command = exec.Command("sh")
	}
	command.Stdin = strings.NewReader(content)
	combinedOutput, err := command.CombinedOutput()
	if err != nil {
		return E.Extend(err, string(combinedOutput))
	}
	return nil
}
