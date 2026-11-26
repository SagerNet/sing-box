//go:build with_tailscale && !windows

package tailssh

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	gliderssh "github.com/sagernet/gliderssh"
	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
)

func isPrivilegedUser() bool {
	return os.Getuid() == 0
}

func requestedUserMatchesProcess(localUser *adapter.PlatformUser) (bool, error) {
	return localUser.Uid == os.Getuid() && localUser.Gid == os.Getgid(), nil
}

func verifyShellIdentity(platformInterface adapter.PlatformInterface, localUser *adapter.PlatformUser) error {
	if platformInterface != nil && platformInterface.UsePlatformShell() {
		return nil
	}
	match, err := requestedUserMatchesProcess(localUser)
	if err != nil {
		return err
	}
	if match {
		return nil
	}
	if !isPrivilegedUser() {
		return E.New("switching to local user ", localUser.Username, " requires sing-box to run as root")
	}
	return checkSwitchUserPrivilege(localUser)
}

func systemHostKeyPath() string {
	return "/etc/ssh/ssh_host_ed25519_key"
}

func defaultPathEnv(_ adapter.PlatformInterface) string {
	return "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
}

func userSocketDirectories(localUser *adapter.PlatformUser) []string {
	return gliderssh.UserSocketDirectories(localUser.HomeDir, strconv.Itoa(localUser.Uid))
}

type agentListener struct {
	net.Listener
}

func (l *agentListener) Close() error {
	err := l.Listener.Close()
	agentDir := filepath.Dir(l.Addr().String())
	if filepath.Dir(agentDir) == filepath.Clean(os.TempDir()) && strings.HasPrefix(filepath.Base(agentDir), "auth-agent") {
		os.RemoveAll(agentDir)
	}
	return err
}

func newAgentListener(localUser *adapter.PlatformUser) (net.Listener, error) {
	gliderListener, err := gliderssh.NewAgentListener()
	if err != nil {
		return nil, err
	}
	listener := &agentListener{Listener: gliderListener}
	socketPath := listener.Addr().String()
	if localUser.Uid < 0 || localUser.Uid == os.Getuid() {
		return listener, nil
	}
	err = os.Chmod(socketPath, 0o600)
	if err != nil {
		listener.Close()
		return nil, err
	}
	err = os.Chown(socketPath, localUser.Uid, localUser.Gid)
	if err != nil {
		listener.Close()
		return nil, err
	}
	err = os.Chown(filepath.Dir(socketPath), localUser.Uid, localUser.Gid)
	if err != nil {
		listener.Close()
		return nil, err
	}
	return listener, nil
}

func chownUserSocket(listener net.Listener, localUser *adapter.PlatformUser) error {
	if localUser.Uid < 0 || localUser.Uid == os.Getuid() {
		return nil
	}
	return os.Lchown(listener.Addr().String(), localUser.Uid, localUser.Gid)
}

func privilegedPortRestricted(localUser *adapter.PlatformUser) bool {
	return !isRootUser(localUser)
}

func platformEnvironment(_ *adapter.PlatformUser) []string {
	return nil
}

func sftpCommand(sftpPath, _ string) string {
	return sftpPath + " 2>/dev/null"
}

func sshSignalToSyscall(sig gliderssh.Signal) int {
	switch sig {
	case gliderssh.SIGABRT:
		return int(syscall.SIGABRT)
	case gliderssh.SIGALRM:
		return int(syscall.SIGALRM)
	case gliderssh.SIGFPE:
		return int(syscall.SIGFPE)
	case gliderssh.SIGHUP:
		return int(syscall.SIGHUP)
	case gliderssh.SIGILL:
		return int(syscall.SIGILL)
	case gliderssh.SIGINT:
		return int(syscall.SIGINT)
	case gliderssh.SIGKILL:
		return int(syscall.SIGKILL)
	case gliderssh.SIGPIPE:
		return int(syscall.SIGPIPE)
	case gliderssh.SIGQUIT:
		return int(syscall.SIGQUIT)
	case gliderssh.SIGSEGV:
		return int(syscall.SIGSEGV)
	case gliderssh.SIGTERM:
		return int(syscall.SIGTERM)
	case gliderssh.SIGUSR1:
		return int(syscall.SIGUSR1)
	case gliderssh.SIGUSR2:
		return int(syscall.SIGUSR2)
	default:
		return 0
	}
}
