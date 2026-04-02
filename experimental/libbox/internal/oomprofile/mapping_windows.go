//go:build windows

package oomprofile

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

func (b *profileBuilder) readMapping() {
	snapshot, err := createModuleSnapshot()
	if err != nil {
		b.addMappingEntry(0, 0, 0, "", "", true)
		return
	}
	defer windows.CloseHandle(snapshot)

	var module windows.ModuleEntry32
	module.Size = uint32(windows.SizeofModuleEntry32)
	err = windows.Module32First(snapshot, &module)
	if err != nil {
		b.addMappingEntry(0, 0, 0, "", "", true)
		return
	}
	for err == nil {
		exe := windows.UTF16ToString(module.ExePath[:])
		b.addMappingEntry(
			uint64(module.ModBaseAddr),
			uint64(module.ModBaseAddr)+uint64(module.ModBaseSize),
			0,
			exe,
			peBuildID(exe),
			false,
		)
		err = windows.Module32Next(snapshot, &module)
	}
}

func createModuleSnapshot() (windows.Handle, error) {
	for {
		snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPMODULE|windows.TH32CS_SNAPMODULE32, uint32(windows.GetCurrentProcessId()))
		var errno windows.Errno
		if err != nil && errors.As(err, &errno) && errno == windows.ERROR_BAD_LENGTH {
			continue
		}
		return snapshot, err
	}
}

func peBuildID(file string) string {
	info, err := os.Stat(file)
	if err != nil {
		return file
	}
	return file + info.ModTime().String()
}
