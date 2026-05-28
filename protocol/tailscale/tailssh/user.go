//go:build with_gvisor

package tailssh

import (
	"github.com/sagernet/sing-box/adapter"
)

func resolveLocalUser(platformInterface adapter.PlatformInterface, username string) (*adapter.PlatformUser, error) {
	var (
		localUser *adapter.PlatformUser
		err       error
	)
	if platformInterface != nil && platformInterface.UsePlatformShell() {
		localUser, err = platformInterface.LookupUser(username)
	} else {
		localUser, err = resolveLocalUserNative(username)
	}
	if err != nil {
		return nil, err
	}
	if localUser.Shell == "" {
		localUser.Shell = defaultShell()
	}
	return localUser, nil
}
