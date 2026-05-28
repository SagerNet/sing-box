//go:build with_gvisor

package tailssh

import (
	"io"

	"github.com/sagernet/sing-box/adapter"
)

type shellBackend interface {
	OpenSession(request shellRequest) (shellSession, error)
	Close() error
}

type shellRequest struct {
	User    *adapter.PlatformUser
	Command string
	Env     []string
	Term    string
	Rows    uint16
	Cols    uint16
}

type shellSession interface {
	io.ReadWriteCloser
	// CloseWrite signals EOF on the child's stdin without tearing down the
	// session, so programs that read stdin to EOF can finish normally.
	CloseWrite() error
	Resize(rows, cols uint16) error
	Signal(sig int) error
	Wait() (exitStatus uint32, err error)
}
