//go:build !unix

package srs

import (
	"os"

	E "github.com/sagernet/sing/common/exceptions"
)

func mmapFile(file *os.File, size int) ([]byte, func(), error) {
	return nil, nil, E.New("rule-set mmap is not supported on this platform")
}
