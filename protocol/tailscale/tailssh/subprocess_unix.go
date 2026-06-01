//go:build unix && !ios

package tailssh

import (
	"os"
	"os/exec"
	"runtime"
	"syscall"

	E "github.com/sagernet/sing/common/exceptions"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

func StartPtyProcess(shell string, args, env []string, dir string, uid, gid int, groups []int, rows, cols uint16) (*os.File, *os.Process, error) {
	cmd := exec.Command(shell)
	cmd.Args = args
	cmd.Dir = dir
	cmd.Env = env
	attrs := &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
		Ctty:    0,
	}
	setCredential(attrs, uid, gid, groups)
	var size *pty.Winsize
	if rows > 0 && cols > 0 {
		size = &pty.Winsize{Rows: rows, Cols: cols}
	}
	master, err := pty.StartWithAttrs(cmd, size, attrs)
	if err != nil {
		return nil, nil, err
	}
	return master, cmd.Process, nil
}

func StartSocketpairProcess(shell string, args, env []string, dir string, uid, gid int, groups []int) (*os.File, *os.Process, error) {
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		return nil, nil, E.Cause(err, "socketpair")
	}
	syscall.CloseOnExec(fds[0])
	syscall.CloseOnExec(fds[1])
	childFile := os.NewFile(uintptr(fds[1]), "socketpair-child")
	cmd := exec.Command(shell)
	cmd.Args = args
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin = childFile
	cmd.Stdout = childFile
	cmd.Stderr = childFile
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
	setCredential(cmd.SysProcAttr, uid, gid, groups)
	err = cmd.Start()
	childFile.Close()
	if err != nil {
		syscall.Close(fds[0])
		return nil, nil, err
	}
	return os.NewFile(uintptr(fds[0]), "socketpair-parent"), cmd.Process, nil
}

func setCredential(attr *syscall.SysProcAttr, uid, gid int, groups []int) {
	if uid < 0 {
		return
	}
	// Skip only when the target identity already matches the server: a non-root
	// server cannot setgroups/setgid, so attempting it would only fail the exec.
	// When the gid differs (a privileged server dropping to another group) we
	// still apply the credential so supplementary groups are reset.
	if uid == os.Getuid() && gid == os.Getgid() {
		return
	}
	// macOS rejects setgroups with more than 16 groups (EINVAL), which fails the
	// exec; cap to the first 16.
	if runtime.GOOS == "darwin" && len(groups) > 16 {
		groups = groups[:16]
	}
	cred := &syscall.Credential{
		Uid: uint32(uid),
		Gid: uint32(gid),
	}
	// Always call setgroups when dropping privileges: an empty slice clears the
	// parent's supplementary groups. Leaving NoSetGroups set here would make a
	// child dropped from root retain root's supplementary groups (wheel/sudo/...).
	cred.Groups = make([]uint32, len(groups))
	for i, g := range groups {
		cred.Groups[i] = uint32(g)
	}
	attr.Credential = cred
}

func SetWinsize(fd int, rows, cols uint16) error {
	return unix.IoctlSetWinsize(fd, unix.TIOCSWINSZ, &unix.Winsize{Row: rows, Col: cols})
}
