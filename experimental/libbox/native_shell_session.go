//go:build linux || android || darwin || ios

package libbox

import (
	"github.com/sagernet/sing-box/protocol/tailscale/tailssh"
	"github.com/sagernet/sing/common"
)

type nativeShellSession struct {
	shell *tailssh.Shell
}

func OpenNativeShellSession(
	shell, cwd string,
	args, environ StringIterator,
	term string,
	rows, cols, uid, gid int32,
	groups Int32Iterator,
) (ShellSession, error) {
	sh, err := tailssh.OpenPtyShell(
		shell,
		iteratorToArray[string](args),
		iteratorToArray[string](environ),
		cwd,
		int(uid), int(gid),
		common.Map(iteratorToArray[int32](groups), func(g int32) int { return int(g) }),
		uint16(rows), uint16(cols),
	)
	if err != nil {
		return nil, err
	}
	return &nativeShellSession{shell: sh}, nil
}

func OpenNativePipeSession(
	shell, cwd string,
	args, environ StringIterator,
	uid, gid int32,
	groups Int32Iterator,
) (ShellSession, error) {
	sh, err := tailssh.OpenSocketpairShell(
		shell,
		iteratorToArray[string](args),
		iteratorToArray[string](environ),
		cwd,
		int(uid), int(gid),
		common.Map(iteratorToArray[int32](groups), func(g int32) int { return int(g) }),
	)
	if err != nil {
		return nil, err
	}
	return &nativeShellSession{shell: sh}, nil
}

func (s *nativeShellSession) MasterFD() int32 {
	return int32(s.shell.MasterFD())
}

func (s *nativeShellSession) Resize(rows, cols int32) error {
	return s.shell.Resize(uint16(rows), uint16(cols))
}

func (s *nativeShellSession) Signal(sig int32) error {
	return s.shell.Signal(int(sig))
}

func (s *nativeShellSession) WaitExit() (int32, error) {
	status, err := s.shell.Wait()
	if err != nil {
		return 0, err
	}
	return int32(status), nil
}

func (s *nativeShellSession) Close() error {
	return s.shell.Close()
}
