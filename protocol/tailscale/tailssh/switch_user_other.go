//go:build with_tailscale && !windows && !linux

package tailssh

import "github.com/sagernet/sing-box/adapter"

func checkSwitchUserPrivilege(_ *adapter.PlatformUser) error {
	return nil
}
