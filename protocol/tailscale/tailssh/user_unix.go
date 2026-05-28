//go:build with_gvisor && !windows && !android

package tailssh

import (
	"os"
	"strconv"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/tailscale/util/osuser"
)

func resolveLocalUserNative(username string) (*adapter.PlatformUser, error) {
	sysUser, shell, err := osuser.LookupByUsernameWithShell(username)
	if err != nil {
		return nil, err
	}
	uid, err := strconv.Atoi(sysUser.Uid)
	if err != nil {
		return nil, err
	}
	gid, err := strconv.Atoi(sysUser.Gid)
	if err != nil {
		return nil, err
	}
	var groups []int
	groupIDs, err := osuser.GetGroupIds(sysUser)
	if err == nil {
		groups = make([]int, 0, len(groupIDs))
		for _, raw := range groupIDs {
			g, parseErr := strconv.Atoi(raw)
			if parseErr != nil {
				continue
			}
			groups = append(groups, g)
		}
	}
	if shell == "" {
		shell = defaultShell()
	}
	return &adapter.PlatformUser{
		Username: sysUser.Username,
		Uid:      uid,
		Gid:      gid,
		HomeDir:  sysUser.HomeDir,
		Shell:    shell,
		Groups:   groups,
	}, nil
}

func defaultShell() string {
	for _, shell := range []string{"/bin/zsh", "/bin/bash", "/bin/sh"} {
		_, err := os.Stat(shell)
		if err == nil {
			return shell
		}
	}
	return "/bin/sh"
}
