package hosts

import _ "unsafe"

var DefaultPath = getSystemDirectory() + "/Drivers/etc/hosts"

//go:linkname getSystemDirectory internal/syscall/windows.GetSystemDirectory
func getSystemDirectory() string
