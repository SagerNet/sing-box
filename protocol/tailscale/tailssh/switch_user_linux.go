//go:build with_gvisor && linux

package tailssh

import (
	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/unix"
)

func checkSwitchUserPrivilege(localUser *adapter.PlatformUser) error {
	header := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	var data [2]unix.CapUserData
	err := unix.Capget(&header, &data[0])
	if err != nil {
		return E.Cause(err, "query process capabilities")
	}
	const required = 1<<unix.CAP_SETUID | 1<<unix.CAP_SETGID
	if data[0].Effective&required != required {
		return E.New("switching to local user ", localUser.Username, " requires CAP_SETUID and CAP_SETGID")
	}
	return nil
}
