//go:build with_gvisor && windows

package tailssh

import (
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"os/user"
	"strings"

	gliderssh "github.com/sagernet/gliderssh"
	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/tailscale/util/winutil"

	winio "github.com/tailscale/go-winio"
	"golang.org/x/sys/windows"
)

func isPrivilegedUser() bool {
	return winutil.IsCurrentProcessElevated()
}

func requestedUserMatchesProcess(localUser *adapter.PlatformUser) (bool, error) {
	tokenUser, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return false, E.Cause(err, "query process token user")
	}
	requested, err := user.Lookup(localUser.Username)
	if err != nil {
		return false, E.Cause(err, "lookup requested user")
	}
	// On Windows os/user reports SIDs in the Uid field.
	return strings.EqualFold(tokenUser.User.Sid.String(), requested.Uid), nil
}

func verifyShellIdentity(platformInterface adapter.PlatformInterface, localUser *adapter.PlatformUser) error {
	if platformInterface != nil && platformInterface.UsePlatformShell() {
		_, loaded := platformInterface.(windowsUserTokenProvider)
		if loaded {
			return nil
		}
	}
	match, err := requestedUserMatchesProcess(localUser)
	if err != nil {
		return err
	}
	if !match {
		return E.New("Windows SSH sessions run as the sing-box process identity; mapping to a different local user (", localUser.Username, ") requires impersonation, which is not implemented")
	}
	return nil
}

func systemHostKeyPath() string {
	return ""
}

func defaultPathEnv(platformInterface adapter.PlatformInterface) string {
	if platformInterface != nil && platformInterface.UsePlatformShell() {
		return ""
	}
	systemRoot := os.Getenv("SystemRoot")
	return systemRoot + `\system32;` + systemRoot + `;` + systemRoot + `\System32\Wbem`
}

func userSocketDirectories(localUser *adapter.PlatformUser) []string {
	return []string{localUser.HomeDir, os.TempDir()}
}

func newAgentListener(localUser *adapter.PlatformUser) (net.Listener, error) {
	requestedUser, err := user.Lookup(localUser.Username)
	if err != nil {
		return nil, E.Cause(err, "lookup requested user")
	}
	pipePath := `\\.\pipe\sing-box-tailssh-agent-` + rand.Text()
	securityDescriptor := fmt.Sprintf(`D:P(A;;GA;;;SY)(A;;GRGW;;;%s)`, requestedUser.Uid)
	listener, err := winio.ListenPipe(pipePath, &winio.PipeConfig{
		SecurityDescriptor: securityDescriptor,
		InputBufferSize:    64 * 1024,
		OutputBufferSize:   64 * 1024,
	})
	if err != nil {
		return nil, E.Cause(err, "listen on agent pipe")
	}
	return listener, nil
}

func platformEnvironment(localUser *adapter.PlatformUser) []string {
	var env []string
	env = append(env, "USERPROFILE="+localUser.HomeDir)
	drive, path, found := strings.Cut(localUser.HomeDir, `\`)
	if found && len(drive) == 2 && drive[1] == ':' {
		env = append(env, "HOMEDRIVE="+drive)
		env = append(env, `HOMEPATH=\`+path)
	}
	env = append(env, "SYSTEMROOT="+os.Getenv("SystemRoot"))
	return env
}

func sftpCommand(sftpPath, shell string) string {
	if isPowerShell(shell) {
		return `& "` + sftpPath + `"`
	}
	return `"` + sftpPath + `"`
}

func sshSignalToSyscall(sig gliderssh.Signal) int {
	switch sig {
	case gliderssh.SIGINT:
		return 2
	case gliderssh.SIGTERM:
		return 15
	case gliderssh.SIGKILL:
		return 9
	default:
		return 0
	}
}
