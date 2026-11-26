//go:build with_tailscale

package tailssh

import (
	"github.com/sagernet/sing-box/adapter"
)

func (s *Server) lookupUser(username string) (*adapter.PlatformUser, error) {
	var (
		localUser *adapter.PlatformUser
		err       error
	)
	if s.platformInterface != nil && s.platformInterface.UsePlatformShell() {
		localUser, err = s.platformInterface.LookupUser(username)
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
