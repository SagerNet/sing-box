//go:build !windows

package oomkiller

import (
	"os"
	"syscall"
)

func lockDraft(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return nil, err
	}
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func unlockDraft(file *os.File) {
	syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	file.Close()
}
