//go:build !linux

package tun

import (
	"os"
)

func Open(name string) (uintptr, error) {
	return 0, os.ErrInvalid
}
