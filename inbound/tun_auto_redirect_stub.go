//go:build !linux

package inbound

import (
	"os"

	E "github.com/sagernet/sing/common/exceptions"
)

type tunAutoRedirect struct{}

func newAutoRedirect(t *Tun) (*tunAutoRedirect, error) {
	return nil, E.New("only supported on linux")
}

func (t *tunAutoRedirect) Start() error {
	return os.ErrInvalid
}

func (t *tunAutoRedirect) Close() error {
	return os.ErrInvalid
}
