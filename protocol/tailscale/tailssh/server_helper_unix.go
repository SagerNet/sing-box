//go:build with_gvisor && !windows

package tailssh

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	gliderssh "github.com/sagernet/gliderssh"
	"github.com/sagernet/sing-box/adapter"
)

func isPrivilegedUser() bool {
	return os.Getuid() == 0
}

func requestedUserMatchesProcess(localUser *adapter.PlatformUser) (bool, error) {
	return localUser.Uid == os.Getuid() && localUser.Gid == os.Getgid(), nil
}

// verifyShellIdentity is a no-op on Unix: spawned shells and sftp-server drop to the
// requested user via setCredential, so the child already runs as that user.
func verifyShellIdentity(_ adapter.PlatformInterface, _ *adapter.PlatformUser) error {
	return nil
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

func newAgentListener(localUser *adapter.PlatformUser) (net.Listener, error) {
	listener, err := gliderssh.NewAgentListener()
	if err != nil {
		return nil, err
	}
	socketPath := listener.Addr().String()
	if localUser.Uid < 0 || localUser.Uid == os.Getuid() {
		return listener, nil
	}
	err = os.Chown(socketPath, localUser.Uid, localUser.Gid)
	if err != nil {
		listener.Close()
		return nil, err
	}
	err = os.Chmod(socketPath, 0o600)
	if err != nil {
		listener.Close()
		return nil, err
	}
	// Make the MkdirTemp parent traversable so the dropped-privilege child can
	// reach the socket.
	err = os.Chmod(filepath.Dir(socketPath), 0o755)
	if err != nil {
		listener.Close()
		return nil, err
	}
	return listener, nil
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
