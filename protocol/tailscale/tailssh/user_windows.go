//go:build with_gvisor && windows

package tailssh

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"

	"github.com/sagernet/sing-box/adapter"
)

func resolveLocalUserNative(username string) (*adapter.PlatformUser, error) {
	sysUser, err := user.Lookup(username)
	if err != nil {
		return nil, err
	}
	return &adapter.PlatformUser{
		Username: sysUser.Username,
		// Windows has no numeric uid/gid; these are placeholders (-1). Identity
		// enforcement compares the token SID via requestedUserMatchesProcess, not
		// these fields.
		Uid:     os.Getuid(),
		Gid:     os.Getgid(),
		HomeDir: sysUser.HomeDir,
		Shell:   defaultShell(),
	}, nil
}

func defaultShell() string {
	for _, name := range []string{"pwsh", "powershell", "cmd"} {
		shellPath, err := exec.LookPath(name)
		if err == nil {
			return shellPath
		}
	}
	return filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
}
