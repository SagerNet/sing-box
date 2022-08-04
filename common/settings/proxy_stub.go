//go:build !(windows || linux || darwin)

package settings

import (
	"os"

	"github.com/sagernet/sing-box/adapter"
)

func SetSystemProxy(router adapter.Router, port uint16, isMixed bool) (func() error, error) {
	return nil, os.ErrInvalid
}
