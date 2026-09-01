//go:build !linux

package libbox

import "os"

func NewAutoRedirectService(options []byte, handler AutoRedirectHandler) (AutoRedirectSession, error) {
	return nil, os.ErrInvalid
}
