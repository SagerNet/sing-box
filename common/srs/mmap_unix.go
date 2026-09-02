//go:build unix

package srs

import (
	"os"

	"golang.org/x/sys/unix"
)

func mmapFile(file *os.File, size int) ([]byte, func(), error) {
	data, err := unix.Mmap(int(file.Fd()), 0, size, unix.PROT_READ, unix.MAP_PRIVATE)
	if err != nil {
		return nil, nil, err
	}
	os.Remove(file.Name())
	return data, func() {
		unix.Munmap(data)
	}, nil
}
