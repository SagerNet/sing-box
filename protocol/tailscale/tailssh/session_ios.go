//go:build with_gvisor && ios

package tailssh

import (
	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
)

func selectShellBackend(platformInterface adapter.PlatformInterface) shellBackend {
	if platformInterface != nil && platformInterface.UsePlatformShell() {
		return &platformShellBackend{platform: platformInterface}
	}
	return iosShellBackend{}
}

func CheckServerSupport(platformInterface adapter.PlatformInterface) (string, error) {
	if platformInterface != nil && platformInterface.UsePlatformShell() {
		return "", nil
	}
	return "", E.New("SSH server is not supported on iOS and tvOS")
}

type iosShellBackend struct{}

func (iosShellBackend) OpenSession(_ shellRequest) (shellSession, error) {
	return nil, E.New("shell sessions are not supported on iOS and tvOS")
}

func (iosShellBackend) Close() error {
	return nil
}

func lookupSFTPServer(_ adapter.PlatformInterface) (string, error) {
	return "", E.New("sftp is not supported on iOS and tvOS")
}
