//go:build go1.21 && !without_badtls && with_utls

package badtls

import (
	"net"
	_ "unsafe"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/utls"
)

func init() {
	tlsRegistry = append(tlsRegistry, func(conn net.Conn) (loaded bool, tlsReadRecord func() error, tlsHandlePostHandshakeMessage func() error) {
		tlsConn, loaded := common.Cast[*tls.UConn](conn)
		if !loaded {
			return
		}
		return true, func() error {
				return utlsReadRecord(tlsConn.Conn)
			}, func() error {
				return utlsHandlePostHandshakeMessage(tlsConn.Conn)
			}
	})
}

//go:linkname utlsReadRecord github.com/sagernet/utls.(*Conn).readRecord
func utlsReadRecord(c *tls.Conn) error

//go:linkname utlsHandlePostHandshakeMessage github.com/sagernet/utls.(*Conn).handlePostHandshakeMessage
func utlsHandlePostHandshakeMessage(c *tls.Conn) error
