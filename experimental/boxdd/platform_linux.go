//go:build linux && !android

package main

import (
	"net/netip"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/settings"
	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/shell"
)

type linuxPlatformInterface struct {
	daemon             *Daemon
	access             sync.Mutex
	ownerUser          *user.User
	systemProxy        *settings.LinuxSystemProxy
	systemProxyEnabled bool
}

func newPlatformInterface(daemonInstance *Daemon) (daemonPlatform, error) {
	return &linuxPlatformInterface{
		daemon:             daemonInstance,
		systemProxyEnabled: true,
	}, nil
}

func (p *linuxPlatformInterface) Initialize(networkManager adapter.NetworkManager) error {
	return nil
}

func (p *linuxPlatformInterface) UsePlatformAutoDetectInterfaceControl() bool {
	return false
}

func (p *linuxPlatformInterface) AutoDetectInterfaceControl(fd int) error {
	return os.ErrInvalid
}

func (p *linuxPlatformInterface) UsePlatformInterface() bool {
	return false
}

func (p *linuxPlatformInterface) OpenInterface(options *tun.Options, platformOptions option.TunPlatformOptions) (tun.Tun, error) {
	return nil, os.ErrInvalid
}

func (p *linuxPlatformInterface) ProcessPlatformOptions(options option.TunPlatformOptions) error {
	if options.HTTPProxy == nil || !options.HTTPProxy.Enabled {
		return nil
	}
	httpProxyOptions := options.HTTPProxy
	systemProxy, err := settings.NewLinuxSystemProxy(
		M.ParseSocksaddrHostPort(httpProxyOptions.Server, httpProxyOptions.ServerPort),
		false,
		p.executeAsOwner,
	)
	if err != nil {
		p.daemon.logger.Warn("initialize system proxy: ", err)
		return nil
	}
	p.access.Lock()
	if p.systemProxy != nil {
		p.access.Unlock()
		return E.New("only one enabled `tun.platform.http_proxy` is supported")
	}
	p.systemProxy = systemProxy
	err = p.applySystemProxyLocked()
	if err != nil {
		rollbackError := p.disableSystemProxyLocked()
		p.systemProxy = nil
		p.access.Unlock()
		return E.Errors(E.Cause(err, "set system proxy"), rollbackError)
	}
	p.access.Unlock()
	return nil
}

func (p *linuxPlatformInterface) UsePlatformDefaultInterfaceMonitor() bool {
	return false
}

func (p *linuxPlatformInterface) CreateDefaultInterfaceMonitor(logger logger.Logger) tun.DefaultInterfaceMonitor {
	return nil
}

func (p *linuxPlatformInterface) UsePlatformNetworkInterfaces() bool {
	return false
}

func (p *linuxPlatformInterface) NetworkInterfaces() ([]adapter.NetworkInterface, error) {
	return nil, os.ErrInvalid
}

func (p *linuxPlatformInterface) UnderNetworkExtension() bool {
	return false
}

func (p *linuxPlatformInterface) NetworkExtensionIncludeAllNetworks() bool {
	return false
}

func (p *linuxPlatformInterface) ClearDNSCache() {
}

func (p *linuxPlatformInterface) RequestPermissionForWIFIState() error {
	return nil
}

func (p *linuxPlatformInterface) ReadWIFIState() adapter.WIFIState {
	return adapter.WIFIState{}
}

func (p *linuxPlatformInterface) UsePlatformConnectionOwnerFinder() bool {
	return false
}

func (p *linuxPlatformInterface) FindConnectionOwner(request *adapter.FindConnectionOwnerRequest) (*adapter.ConnectionOwner, error) {
	return nil, os.ErrInvalid
}

func (p *linuxPlatformInterface) UsePlatformWIFIMonitor() bool {
	return false
}

func (p *linuxPlatformInterface) UsePlatformNotification() bool {
	return false
}

func (p *linuxPlatformInterface) SendNotification(notification *adapter.Notification) error {
	return nil
}

func (p *linuxPlatformInterface) MyInterfaceAddress() []netip.Addr {
	return nil
}

func (p *linuxPlatformInterface) UsePlatformNeighborResolver() bool {
	return false
}

func (p *linuxPlatformInterface) StartNeighborMonitor(listener adapter.NeighborUpdateListener) error {
	return os.ErrInvalid
}

func (p *linuxPlatformInterface) CloseNeighborMonitor(listener adapter.NeighborUpdateListener) error {
	return nil
}

func (p *linuxPlatformInterface) UsePlatformShell() bool {
	return false
}

func (p *linuxPlatformInterface) CheckPlatformShell() error {
	return nil
}

func (p *linuxPlatformInterface) OpenShellSession(user *adapter.PlatformUser, command string, environ []string, term string, rows int32, cols int32) (adapter.ShellSession, error) {
	return nil, os.ErrInvalid
}

func (p *linuxPlatformInterface) LookupUser(username string) (*adapter.PlatformUser, error) {
	return nil, os.ErrInvalid
}

func (p *linuxPlatformInterface) LookupSFTPServer() (string, error) {
	return "", os.ErrInvalid
}

func (p *linuxPlatformInterface) ReadSystemSSHHostKey() ([]byte, error) {
	return nil, os.ErrInvalid
}

func (p *linuxPlatformInterface) TailscaleHostname() string {
	return ""
}

func (p *linuxPlatformInterface) UsePlatformBridge() bool {
	return false
}

func (p *linuxPlatformInterface) CreateBridge(options adapter.BridgeOptions) (adapter.BridgeSession, error) {
	return nil, os.ErrInvalid
}

func (p *linuxPlatformInterface) PrepareOwner(identity peerIdentity) error {
	p.access.Lock()
	defer p.access.Unlock()
	if listenAddress != "" {
		return p.applySystemProxyLocked()
	}
	if p.ownerUser != nil && p.ownerUser.Uid == identity.UserID {
		return p.applySystemProxyLocked()
	}
	ownerUser, err := user.LookupId(identity.UserID)
	if err != nil {
		return E.Cause(err, "lookup owner user")
	}
	return p.replaceOwnerLocked(ownerUser)
}

func (p *linuxPlatformInterface) RestoreOwner(state ownerState) error {
	p.access.Lock()
	defer p.access.Unlock()
	if listenAddress != "" {
		return nil
	}
	ownerUser, err := user.LookupId(state.UserID)
	if err != nil {
		return E.Cause(err, "lookup owner user")
	}
	p.ownerUser = ownerUser
	return nil
}

func (p *linuxPlatformInterface) ReleaseOwner() error {
	p.access.Lock()
	defer p.access.Unlock()
	err := p.disableSystemProxyLocked()
	if err != nil {
		return err
	}
	p.ownerUser = nil
	return nil
}

func (p *linuxPlatformInterface) ResetPlatformOptions() error {
	p.access.Lock()
	defer p.access.Unlock()
	err := p.disableSystemProxyLocked()
	if err == nil {
		p.systemProxy = nil
	}
	return err
}

func (p *linuxPlatformInterface) SetSystemProxyPreference(enabled bool) {
	p.access.Lock()
	p.systemProxyEnabled = enabled
	p.access.Unlock()
}

func (p *linuxPlatformInterface) SystemProxyStatus() (*daemon.SystemProxyStatus, error) {
	p.access.Lock()
	defer p.access.Unlock()
	available := p.systemProxy != nil
	return &daemon.SystemProxyStatus{
		Available: available,
		Enabled:   available && p.systemProxyEnabled,
	}, nil
}

func (p *linuxPlatformInterface) SetSystemProxyEnabled(enabled bool) error {
	p.access.Lock()
	defer p.access.Unlock()
	if p.systemProxy == nil {
		if !enabled {
			p.systemProxyEnabled = false
			return nil
		}
		return E.New("the system proxy is not available")
	}
	previousEnabled := p.systemProxyEnabled
	p.systemProxyEnabled = enabled
	err := p.applySystemProxyLocked()
	if err != nil {
		p.systemProxyEnabled = previousEnabled
		rollbackError := p.applySystemProxyLocked()
		return E.Errors(err, rollbackError)
	}
	return nil
}

func (p *linuxPlatformInterface) HandleSessionChange(eventType uint32, sessionID uint32, state ownerState) (uint32, bool, error) {
	return 0, false, nil
}

func (p *linuxPlatformInterface) Close() error {
	p.access.Lock()
	defer p.access.Unlock()
	err := p.disableSystemProxyLocked()
	p.systemProxy = nil
	p.ownerUser = nil
	return err
}

func (p *linuxPlatformInterface) applySystemProxyLocked() error {
	if p.systemProxy == nil {
		return nil
	}
	if p.systemProxyEnabled {
		if p.systemProxy.IsEnabled() || !p.ownerSessionActiveLocked() {
			return nil
		}
		return p.systemProxy.Enable()
	}
	return p.disableSystemProxyLocked()
}

func (p *linuxPlatformInterface) disableSystemProxyLocked() error {
	if p.systemProxy == nil || !p.systemProxy.IsEnabled() || !p.ownerSessionActiveLocked() {
		return nil
	}
	return p.systemProxy.Disable()
}

func (p *linuxPlatformInterface) ownerSessionActiveLocked() bool {
	if listenAddress != "" {
		return true
	}
	if p.ownerUser == nil {
		return false
	}
	_, err := os.Stat(ownerSessionBusPath(p.ownerUser))
	return err == nil
}

func (p *linuxPlatformInterface) executeAsOwner(name string, args ...string) error {
	if listenAddress != "" {
		return shell.Exec(name, args...).Attach().Run()
	}
	ownerUser := p.ownerUser
	if ownerUser == nil {
		return E.New("missing owner session")
	}
	uid, err := strconv.ParseUint(ownerUser.Uid, 10, 32)
	if err != nil {
		return E.Cause(err, "parse owner user ID")
	}
	gid, err := strconv.ParseUint(ownerUser.Gid, 10, 32)
	if err != nil {
		return E.Cause(err, "parse owner group ID")
	}
	searchPath := os.Getenv("PATH")
	if searchPath == "" {
		searchPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	}
	command := exec.Command(name, args...)
	command.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: uint32(uid),
			Gid: uint32(gid),
		},
	}
	command.Env = []string{
		"HOME=" + ownerUser.HomeDir,
		"USER=" + ownerUser.Username,
		"LOGNAME=" + ownerUser.Username,
		"PATH=" + searchPath,
		"XDG_RUNTIME_DIR=" + ownerRuntimeDirectory(ownerUser),
		"DBUS_SESSION_BUS_ADDRESS=unix:path=" + ownerSessionBusPath(ownerUser),
	}
	output, err := command.CombinedOutput()
	if err != nil {
		outputText := strings.TrimSpace(string(output))
		if outputText != "" {
			return E.Cause(err, "execute (", name, ") ", strings.Join(args, " "), ": ", outputText)
		}
		return E.Cause(err, "execute (", name, ") ", strings.Join(args, " "))
	}
	return nil
}

func (p *linuxPlatformInterface) replaceOwnerLocked(ownerUser *user.User) error {
	err := p.disableSystemProxyLocked()
	if err != nil {
		return err
	}
	p.ownerUser = ownerUser
	return p.applySystemProxyLocked()
}

func ownerRuntimeDirectory(ownerUser *user.User) string {
	return "/run/user/" + ownerUser.Uid
}

func ownerSessionBusPath(ownerUser *user.User) string {
	return ownerRuntimeDirectory(ownerUser) + "/bus"
}

var _ daemonPlatform = (*linuxPlatformInterface)(nil)
