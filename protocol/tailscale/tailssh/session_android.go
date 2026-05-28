//go:build with_gvisor && android

package tailssh

import "github.com/sagernet/sing-box/adapter"

func selectShellBackend(platformInterface adapter.PlatformInterface) shellBackend {
	return &platformShellBackend{platform: platformInterface}
}

func CheckServerSupport(platformInterface adapter.PlatformInterface) (string, error) {
	if platformInterface != nil {
		err := platformInterface.CheckPlatformShell()
		if err == nil {
			return "", nil
		}
	}
	return "running without root, SSH sessions are limited to the sing-box user", nil
}

func lookupSFTPServer(platformInterface adapter.PlatformInterface) (string, error) {
	return platformInterface.LookupSFTPServer()
}
