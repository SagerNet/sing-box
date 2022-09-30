//go:build !go1.19 || go1.20

package badtls

import (
	"crypto/tls"
	"os"
)

func Create(conn *tls.Conn) (TLSConn, error) {
	return nil, os.ErrInvalid
}
