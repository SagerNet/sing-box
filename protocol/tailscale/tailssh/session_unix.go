//go:build with_gvisor && unix && !android && !ios

package tailssh

import (
	"os"
	"path/filepath"

	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
)

func selectShellBackend(platformInterface adapter.PlatformInterface) shellBackend {
	if platformInterface != nil && platformInterface.UsePlatformShell() {
		return &platformShellBackend{platform: platformInterface}
	}
	return &directShellBackend{}
}

func CheckServerSupport(platformInterface adapter.PlatformInterface) (string, error) {
	if platformInterface != nil && platformInterface.UnderNetworkExtension() {
		if !platformInterface.UsePlatformShell() {
			return "", E.New("SSH server is not supported in the App Store version of sing-box")
		}
		err := platformInterface.CheckPlatformShell()
		if err != nil {
			return "", E.Cause(err, "missing Root Helper")
		}
		return "", nil
	}
	if !isPrivilegedUser() {
		return "running without root, SSH sessions are limited to the current user", nil
	}
	return "", nil
}

type directShellBackend struct{}

func (b *directShellBackend) OpenSession(request shellRequest) (shellSession, error) {
	shell := request.User.Shell
	var args []string
	if request.Command != "" {
		args = []string{shell, "-c", request.Command}
	} else {
		args = []string{"-" + filepath.Base(shell)}
	}
	if request.Term != "" {
		return OpenPtyShell(shell, args, request.Env, request.User.HomeDir, request.User.Uid, request.User.Gid, request.User.Groups, request.Rows, request.Cols)
	}
	return OpenSocketpairShell(shell, args, request.Env, request.User.HomeDir, request.User.Uid, request.User.Gid, request.User.Groups)
}

func (b *directShellBackend) Close() error {
	return nil
}

func lookupSFTPServer(_ adapter.PlatformInterface) (string, error) {
	for _, path := range []string{
		"/usr/libexec/sftp-server",
		"/usr/lib/openssh/sftp-server",
		"/usr/lib/ssh/sftp-server",
		"/usr/libexec/openssh/sftp-server",
	} {
		_, err := os.Stat(path)
		if err == nil {
			return path, nil
		}
	}
	return "", E.New("sftp-server not found")
}
