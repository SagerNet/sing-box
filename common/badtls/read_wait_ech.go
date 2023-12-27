//go:build go1.21 && !without_badtls && with_ech

package badtls

import (
	"net"
	_ "unsafe"

	"github.com/sagernet/cloudflare-tls"
	"github.com/sagernet/sing/common"
)

func init() {
	tlsRegistry = append(tlsRegistry, func(conn net.Conn) (loaded bool, tlsReadRecord func() error, tlsHandlePostHandshakeMessage func() error) {
		tlsConn, loaded := common.Cast[*tls.Conn](conn)
		if !loaded {
			return
		}
		return true, func() error {
				return echReadRecord(tlsConn)
			}, func() error {
				return echHandlePostHandshakeMessage(tlsConn)
			}
	})
}

//go:linkname echReadRecord github.com/sagernet/cloudflare-tls.(*Conn).readRecord
func echReadRecord(c *tls.Conn) error

//go:linkname echHandlePostHandshakeMessage github.com/sagernet/cloudflare-tls.(*Conn).handlePostHandshakeMessage
func echHandlePostHandshakeMessage(c *tls.Conn) error
