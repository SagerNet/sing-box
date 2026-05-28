//go:build !linux && !android && (!darwin || ios)

package libbox

import (
	E "github.com/sagernet/sing/common/exceptions"
)

func OpenNativeShellSession(
	shell, cwd string,
	args, environ StringIterator,
	term string,
	rows, cols, uid, gid int32,
	groups Int32Iterator,
) (ShellSession, error) {
	return nil, E.New("native shell session not supported on this platform")
}

func OpenNativePipeSession(
	shell, cwd string,
	args, environ StringIterator,
	uid, gid int32,
	groups Int32Iterator,
) (ShellSession, error) {
	return nil, E.New("native pipe session not supported on this platform")
}
