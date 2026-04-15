package hosts

import (
	"path/filepath"
	"sync"

	"golang.org/x/sys/windows"
)

var defaultPath = sync.OnceValues(func() (string, error) {
	systemDirectory, err := windows.GetSystemDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(systemDirectory, "Drivers", "etc", "hosts"), nil
})
