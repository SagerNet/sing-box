//go:build !go1.19 || go1.21

package badtls

import (
	"crypto/tls"
	"os"

	aTLS "github.com/sagernet/sing/common/tls"
)

func Create(conn *tls.Conn) (aTLS.Conn, error) {
	return nil, os.ErrInvalid
}
