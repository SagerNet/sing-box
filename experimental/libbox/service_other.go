//go:build !windows

package libbox

import "syscall"

func dup(fd int) (nfd int, err error) {
	return syscall.Dup(fd)
}
