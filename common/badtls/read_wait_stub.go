//go:build !go1.25 || !badlinkname

package badtls

import (
	"os"

	"github.com/sagernet/sing/common/tls"
)

func NewReadWaitConn(conn tls.Conn) (tls.Conn, error) {
	return nil, os.ErrInvalid
}
