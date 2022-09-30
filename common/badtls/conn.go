package badtls

import (
	"context"
	"crypto/tls"
	"net"
)

type TLSConn interface {
	net.Conn
	HandshakeContext(ctx context.Context) error
	ConnectionState() tls.ConnectionState
}
