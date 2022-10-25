//go:build !debug

package libbox

import (
	"os"
)

type PProfServer struct{}

func NewPProfServer(port int) *PProfServer {
	return &PProfServer{}
}

func (s *PProfServer) Start() error {
	return os.ErrInvalid
}

func (s *PProfServer) Close() error {
	return os.ErrInvalid
}
