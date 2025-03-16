package hosts

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

var DefaultPath string

func init() {
	systemDirectory, err := windows.GetSystemDirectory()
	if err != nil {
		systemDirectory = "C:\\Windows\\System32"
	}
	DefaultPath = filepath.Join(systemDirectory, "Drivers/etc/hosts")
}
