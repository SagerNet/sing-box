//go:build with_gvisor && !windows

package tailssh

import (
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
func verifyShellIdentity(_ *adapter.PlatformUser) error {
	return nil
}

func systemHostKeyPath() string {
	return "/etc/ssh/ssh_host_ed25519_key"
}

func defaultPathEnv() string {
	return "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
}

func userSocketDirectories(localUser *adapter.PlatformUser) []string {
	return gliderssh.UserSocketDirectories(localUser.HomeDir, strconv.Itoa(localUser.Uid))
}

// prepareAgentSocket hands the agent-forwarding socket to the target user so
// SSH_AUTH_SOCK stays reachable after the shell drops privileges. No-op when the
// shell runs as the server identity.
func prepareAgentSocket(socketPath string, uid, gid int) error {
	if uid < 0 || uid == os.Getuid() {
		return nil
	}
	err := os.Chown(socketPath, uid, gid)
	if err != nil {
		return err
	}
	err = os.Chmod(socketPath, 0o600)
	if err != nil {
		return err
	}
	// Make the MkdirTemp parent traversable so the dropped-privilege child can
	// reach the socket.
	return os.Chmod(filepath.Dir(socketPath), 0o755)
}

func platformEnvironment(_ *adapter.PlatformUser) []string {
	return nil
}

func sftpCommand(sftpPath string) string {
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
