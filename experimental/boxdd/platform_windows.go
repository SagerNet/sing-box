//go:build windows

package main

import (
	"context"
	"io"
	"net/netip"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/settings"
	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/tailscale/go-winio"
	"golang.org/x/sys/windows"
)

var regDisablePredefinedCacheEx = windows.NewLazySystemDLL("advapi32.dll").NewProc("RegDisablePredefinedCacheEx")

type windowsPlatformInterface struct {
	daemon             *Daemon
	access             sync.Mutex
	updateAccess       sync.Mutex
	updateInProgress   bool
	daemonSigner       []byte
	ownerUserID        string
	sessionID          uint32
	token              windows.Token
	systemProxy        *settings.WindowsSystemProxy
	systemProxyEnabled bool
}

func newPlatformInterface(daemonInstance *Daemon) (daemonPlatform, error) {
	result, _, _ := regDisablePredefinedCacheEx.Call()
	if result != 0 {
		return nil, E.Cause(syscall.Errno(result), "disable predefined registry handle cache")
	}
	return &windowsPlatformInterface{
		daemon:             daemonInstance,
		systemProxyEnabled: true,
	}, nil
}

func (p *windowsPlatformInterface) Initialize(networkManager adapter.NetworkManager) error {
	return nil
}

func (p *windowsPlatformInterface) UsePlatformAutoDetectInterfaceControl() bool {
	return false
}

func (p *windowsPlatformInterface) AutoDetectInterfaceControl(fd int) error {
	return os.ErrInvalid
}

func (p *windowsPlatformInterface) UsePlatformInterface() bool {
	return false
}

func (p *windowsPlatformInterface) OpenInterface(options *tun.Options, platformOptions option.TunPlatformOptions) (tun.Tun, error) {
	return nil, os.ErrInvalid
}

func (p *windowsPlatformInterface) ProcessPlatformOptions(options option.TunPlatformOptions) error {
	if options.HTTPProxy == nil || !options.HTTPProxy.Enabled {
		return nil
	}
	httpProxyOptions := options.HTTPProxy
	systemProxy, err := settings.NewSystemProxy(
		context.Background(),
		M.ParseSocksaddrHostPort(httpProxyOptions.Server, httpProxyOptions.ServerPort),
		false,
		[]string(httpProxyOptions.BypassDomain),
	)
	if err != nil {
		return E.Cause(err, "initialize system proxy")
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

func (p *windowsPlatformInterface) UsePlatformDefaultInterfaceMonitor() bool {
	return false
}

func (p *windowsPlatformInterface) CreateDefaultInterfaceMonitor(logger logger.Logger) tun.DefaultInterfaceMonitor {
	return nil
}

func (p *windowsPlatformInterface) UsePlatformNetworkInterfaces() bool {
	return false
}

func (p *windowsPlatformInterface) NetworkInterfaces() ([]adapter.NetworkInterface, error) {
	return nil, os.ErrInvalid
}

func (p *windowsPlatformInterface) UnderNetworkExtension() bool {
	return false
}

func (p *windowsPlatformInterface) NetworkExtensionIncludeAllNetworks() bool {
	return false
}

func (p *windowsPlatformInterface) ClearDNSCache() {
}

func (p *windowsPlatformInterface) RequestPermissionForWIFIState() error {
	return nil
}

func (p *windowsPlatformInterface) ReadWIFIState() adapter.WIFIState {
	return adapter.WIFIState{}
}

func (p *windowsPlatformInterface) UsePlatformConnectionOwnerFinder() bool {
	return false
}

func (p *windowsPlatformInterface) FindConnectionOwner(request *adapter.FindConnectionOwnerRequest) (*adapter.ConnectionOwner, error) {
	return nil, os.ErrInvalid
}

func (p *windowsPlatformInterface) UsePlatformWIFIMonitor() bool {
	return false
}

func (p *windowsPlatformInterface) UsePlatformNotification() bool {
	return false
}

func (p *windowsPlatformInterface) SendNotification(notification *adapter.Notification) error {
	return nil
}

func (p *windowsPlatformInterface) MyInterfaceAddress() []netip.Addr {
	return nil
}

func (p *windowsPlatformInterface) UsePlatformNeighborResolver() bool {
	return false
}

func (p *windowsPlatformInterface) StartNeighborMonitor(listener adapter.NeighborUpdateListener) error {
	return os.ErrInvalid
}

func (p *windowsPlatformInterface) CloseNeighborMonitor(listener adapter.NeighborUpdateListener) error {
	return nil
}

func (p *windowsPlatformInterface) UsePlatformShell() bool {
	return listenAddress == ""
}

func (p *windowsPlatformInterface) CheckPlatformShell() error {
	return nil
}

func (p *windowsPlatformInterface) OpenShellSession(user *adapter.PlatformUser, command string, environ []string, term string, rows int32, cols int32) (adapter.ShellSession, error) {
	return nil, os.ErrInvalid
}

func (p *windowsPlatformInterface) LookupUser(username string) (*adapter.PlatformUser, error) {
	requestedUser, err := user.Lookup(username)
	if err != nil {
		return nil, E.Cause(err, "lookup Windows user")
	}
	return &adapter.PlatformUser{
		Username: requestedUser.Username,
		Uid:      os.Getuid(),
		Gid:      os.Getgid(),
		HomeDir:  requestedUser.HomeDir,
	}, nil
}

func (p *windowsPlatformInterface) LookupSFTPServer() (string, error) {
	for _, sftpPath := range []string{
		filepath.Join(os.Getenv("SystemRoot"), "System32", "OpenSSH", "sftp-server.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "OpenSSH", "sftp-server.exe"),
	} {
		_, err := os.Stat(sftpPath)
		if err == nil {
			return sftpPath, nil
		}
	}
	return "", E.New("sftp-server not found")
}

func (p *windowsPlatformInterface) ReadSystemSSHHostKey() ([]byte, error) {
	return nil, os.ErrInvalid
}

func (p *windowsPlatformInterface) TailscaleHostname() string {
	return ""
}

func (p *windowsPlatformInterface) AcquireWindowsUserToken(localUser *adapter.PlatformUser) (windows.Token, io.Closer, error) {
	requestedUser, err := user.Lookup(localUser.Username)
	if err != nil {
		return 0, nil, E.Cause(err, "lookup Windows user")
	}
	return acquireWindowsUserSession(requestedUser)
}

func (p *windowsPlatformInterface) UsePlatformBridge() bool {
	return false
}

func (p *windowsPlatformInterface) CreateBridge(options adapter.BridgeOptions) (adapter.BridgeSession, error) {
	return nil, os.ErrInvalid
}

func (p *windowsPlatformInterface) PrepareOwner(identity peerIdentity) error {
	p.access.Lock()
	defer p.access.Unlock()
	if listenAddress != "" {
		p.ownerUserID = identity.UserID
		p.sessionID = identity.SessionID
		return p.applySystemProxyLocked()
	}
	if p.token != 0 && p.ownerUserID == identity.UserID && p.sessionID == identity.SessionID {
		return p.applySystemProxyLocked()
	}
	token, err := p.daemon.duplicatePeerImpersonationToken(identity)
	if err != nil {
		return err
	}
	err = validateImpersonationToken(token, identity.UserID, identity.SessionID)
	if err != nil {
		token.Close()
		return err
	}
	err = p.replaceOwnerTokenLocked(identity.UserID, identity.SessionID, token)
	if err != nil {
		return err
	}
	return nil
}

func (p *windowsPlatformInterface) RestoreOwner(state ownerState) error {
	p.access.Lock()
	defer p.access.Unlock()
	p.ownerUserID = state.UserID
	p.sessionID = state.SessionID
	if listenAddress != "" {
		return nil
	}
	if state.SessionID == 0 {
		return E.New("missing owner session")
	}
	token, err := querySessionImpersonationToken(state.SessionID)
	if err != nil {
		return err
	}
	err = validateImpersonationToken(token, state.UserID, state.SessionID)
	if err != nil {
		token.Close()
		return err
	}
	p.token = token
	return nil
}

func (p *windowsPlatformInterface) ReleaseOwner() error {
	p.access.Lock()
	defer p.access.Unlock()
	return p.releaseOwnerLocked()
}

func (p *windowsPlatformInterface) ResetPlatformOptions() error {
	p.access.Lock()
	defer p.access.Unlock()
	err := p.disableSystemProxyLocked()
	if err == nil {
		p.systemProxy = nil
	}
	return err
}

func (p *windowsPlatformInterface) SetSystemProxyPreference(enabled bool) {
	p.access.Lock()
	p.systemProxyEnabled = enabled
	p.access.Unlock()
}

func (p *windowsPlatformInterface) SystemProxyStatus() (*daemon.SystemProxyStatus, error) {
	p.access.Lock()
	defer p.access.Unlock()
	available := p.systemProxy != nil
	return &daemon.SystemProxyStatus{
		Available: available,
		Enabled:   available && p.systemProxyEnabled,
	}, nil
}

func (p *windowsPlatformInterface) SetSystemProxyEnabled(enabled bool) error {
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

func (p *windowsPlatformInterface) HandleSessionChange(eventType uint32, sessionID uint32, state ownerState) (uint32, bool, error) {
	p.access.Lock()
	defer p.access.Unlock()
	if eventType == windows.WTS_SESSION_LOGOFF {
		if p.sessionID != sessionID {
			return 0, false, nil
		}
		return 0, false, p.releaseOwnerLocked()
	}
	if eventType != windows.WTS_SESSION_LOGON &&
		eventType != windows.WTS_CONSOLE_CONNECT &&
		eventType != windows.WTS_REMOTE_CONNECT &&
		eventType != windows.WTS_SESSION_UNLOCK {
		return 0, false, nil
	}
	token, err := querySessionImpersonationToken(sessionID)
	if err != nil {
		return 0, false, err
	}
	userID, tokenSessionID, err := impersonationTokenIdentity(token)
	if err != nil {
		token.Close()
		return 0, false, err
	}
	if userID != state.UserID || tokenSessionID != sessionID {
		token.Close()
		return 0, false, nil
	}
	err = p.replaceOwnerTokenLocked(userID, sessionID, token)
	if err != nil {
		return 0, false, err
	}
	return sessionID, true, nil
}

func (d *Daemon) handlePlatformSessionChange(eventType uint32, sessionID uint32) error {
	d.lifecycleAccess.Lock()
	defer d.lifecycleAccess.Unlock()
	if d.closed || d.platform == nil {
		return nil
	}
	state, err := loadOwnerState()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	newSessionID, changed, err := d.platform.HandleSessionChange(eventType, sessionID, state)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	return saveOwner(state.UserID, newSessionID)
}

func (p *windowsPlatformInterface) Close() error {
	p.access.Lock()
	defer p.access.Unlock()
	systemProxyError := p.disableSystemProxyLocked()
	p.systemProxy = nil
	ownerError := p.closeOwnerTokenLocked()
	return E.Errors(systemProxyError, ownerError)
}

func (p *windowsPlatformInterface) applySystemProxyLocked() error {
	if p.systemProxy == nil {
		return nil
	}
	if p.systemProxyEnabled {
		if p.systemProxy.IsEnabled() {
			return nil
		}
		return p.runUserOperationLocked(p.systemProxy.Enable)
	}
	return p.disableSystemProxyLocked()
}

func (p *windowsPlatformInterface) disableSystemProxyLocked() error {
	if p.systemProxy == nil || !p.systemProxy.IsEnabled() {
		return nil
	}
	return p.runUserOperationLocked(p.systemProxy.Disable)
}

func (p *windowsPlatformInterface) runUserOperationLocked(operation func() error) error {
	if listenAddress != "" {
		return operation()
	}
	if p.token == 0 {
		return nil
	}
	return runImpersonated(p.token, operation)
}

func (p *windowsPlatformInterface) replaceOwnerTokenLocked(userID string, sessionID uint32, token windows.Token) error {
	err := p.disableSystemProxyLocked()
	if err != nil {
		return E.Errors(err, token.Close())
	}
	err = p.closeOwnerTokenLocked()
	if err != nil {
		return E.Errors(err, token.Close())
	}
	p.ownerUserID = userID
	p.sessionID = sessionID
	p.token = token
	err = p.applySystemProxyLocked()
	if err != nil {
		return E.Errors(err, p.closeOwnerTokenLocked())
	}
	return nil
}

func (p *windowsPlatformInterface) releaseOwnerLocked() error {
	err := p.disableSystemProxyLocked()
	if err != nil {
		return err
	}
	err = p.closeOwnerTokenLocked()
	p.ownerUserID = ""
	p.sessionID = 0
	return err
}

func (p *windowsPlatformInterface) closeOwnerTokenLocked() error {
	if p.token == 0 {
		return nil
	}
	err := p.token.Close()
	p.token = 0
	return err
}

func runImpersonated(token windows.Token, operation func() error) error {
	result := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		err := windows.SetThreadToken(nil, token)
		if err != nil {
			runtime.UnlockOSThread()
			result <- E.Cause(err, "impersonate owner")
			return
		}
		operationError := operation()
		revertError := windows.RevertToSelf()
		if revertError == nil {
			runtime.UnlockOSThread()
		} else {
			revertError = E.Cause(revertError, "revert owner impersonation")
		}
		result <- E.Errors(operationError, revertError)
	}()
	return <-result
}

func querySessionImpersonationToken(sessionID uint32) (windows.Token, error) {
	var primaryToken windows.Token
	err := winio.RunWithPrivileges([]string{seTcbPrivilege}, func() error {
		return windows.WTSQueryUserToken(sessionID, &primaryToken)
	})
	if err != nil {
		return 0, E.Cause(err, "query session user token")
	}
	defer primaryToken.Close()
	return duplicateImpersonationToken(primaryToken)
}

func duplicateImpersonationToken(token windows.Token) (windows.Token, error) {
	var duplicatedToken windows.Token
	err := windows.DuplicateTokenEx(
		token,
		windows.TOKEN_QUERY|windows.TOKEN_IMPERSONATE,
		nil,
		windows.SecurityImpersonation,
		windows.TokenImpersonation,
		&duplicatedToken,
	)
	if err != nil {
		return 0, E.Cause(err, "duplicate owner impersonation token")
	}
	return duplicatedToken, nil
}

func validateImpersonationToken(token windows.Token, expectedUserID string, expectedSessionID uint32) error {
	userID, sessionID, err := impersonationTokenIdentity(token)
	if err != nil {
		return err
	}
	if userID != expectedUserID || sessionID != expectedSessionID {
		return E.New("owner token identity does not match authenticated application")
	}
	return nil
}

func impersonationTokenIdentity(token windows.Token) (string, uint32, error) {
	user, err := token.GetTokenUser()
	if err != nil {
		return "", 0, E.Cause(err, "query owner token user")
	}
	userID := user.User.Sid.String()
	if userID == "" {
		return "", 0, E.New("owner token has an invalid user SID")
	}
	var sessionID uint32
	var returnLength uint32
	err = windows.GetTokenInformation(
		token,
		windows.TokenSessionId,
		(*byte)(unsafe.Pointer(&sessionID)),
		uint32(unsafe.Sizeof(sessionID)),
		&returnLength,
	)
	if err != nil {
		return "", 0, E.Cause(err, "query owner token session")
	}
	return userID, sessionID, nil
}

var _ daemonPlatform = (*windowsPlatformInterface)(nil)
