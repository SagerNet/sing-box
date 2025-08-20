//go:build go1.21 && !without_badtls && with_utls

package badtls

import (
	"net"
	_ "unsafe"

	"github.com/metacubex/utls"
)

func init() {
	tlsRegistry = append(tlsRegistry, func(conn net.Conn) (loaded bool, tlsReadRecord func() error, tlsHandlePostHandshakeMessage func() error) {
		switch tlsConn := conn.(type) {
		case *tls.UConn:
			return true, func() error {
					return utlsReadRecord(tlsConn.Conn)
				}, func() error {
					return utlsHandlePostHandshakeMessage(tlsConn.Conn)
				}
		case *tls.Conn:
			return true, func() error {
					return utlsReadRecord(tlsConn)
				}, func() error {
					return utlsHandlePostHandshakeMessage(tlsConn)
				}
		}
		return
	})
}

//go:linkname utlsReadRecord github.com/metacubex/utls.(*Conn).readRecord
func utlsReadRecord(c *tls.Conn) error

//go:linkname utlsHandlePostHandshakeMessage github.com/metacubex/utls.(*Conn).handlePostHandshakeMessage
func utlsHandlePostHandshakeMessage(c *tls.Conn) error
