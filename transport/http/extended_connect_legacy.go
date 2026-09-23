//go:build !go1.27 || http2legacy

package http

import (
	_ "unsafe"

	_ "golang.org/x/net/http2"
)

const extendedConnectAvailable = true

//go:linkname disableExtendedConnectProtocol golang.org/x/net/http2.disableExtendedConnectProtocol
var disableExtendedConnectProtocol bool

func init() {
	disableExtendedConnectProtocol = false
}
