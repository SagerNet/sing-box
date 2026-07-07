package bridge

import (
	"unsafe"
)

//go:linkname unixIoctlPtr golang.org/x/sys/unix.ioctlPtr
func unixIoctlPtr(fd int, request uint, arg unsafe.Pointer) error

//go:linkname unixSysctl golang.org/x/sys/unix.sysctl
func unixSysctl(mib []int32, old *byte, oldLen *uintptr, newValue *byte, newLen uintptr) error
