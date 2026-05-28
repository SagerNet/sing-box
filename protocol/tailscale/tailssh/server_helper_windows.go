//go:build with_gvisor && windows

package tailssh

import (
	"os"
	"os/user"
	"strings"

	gliderssh "github.com/sagernet/gliderssh"
	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/tailscale/util/winutil"

	"golang.org/x/sys/windows"
)

func isPrivilegedUser() bool {
	return winutil.IsCurrentProcessElevated()
}

// requestedUserMatchesProcess reports whether the ACL-mapped user is the same Windows
// account the sing-box process runs as. Windows has no impersonation wired up, so a
// session always runs with the process identity; this is the only case where the
// identity it runs as equals the requested one.
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

// verifyShellIdentity refuses a spawned shell/SFTP session whose ACL-mapped user differs
// from the process identity it would actually run as, since Windows has no impersonation.
func verifyShellIdentity(localUser *adapter.PlatformUser) error {
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

func defaultPathEnv() string {
	systemRoot := os.Getenv("SystemRoot")
	return systemRoot + `\system32;` + systemRoot + `;` + systemRoot + `\System32\Wbem`
}

func userSocketDirectories(localUser *adapter.PlatformUser) []string {
	return []string{localUser.HomeDir, os.TempDir()}
}

// prepareAgentSocket is a no-op on Windows: shells run as the server identity, so
// the agent socket needs no ownership change.
func prepareAgentSocket(_ string, _, _ int) error {
	return nil
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

func sftpCommand(sftpPath string) string {
	return sftpPath
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
