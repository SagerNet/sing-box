//go:build go1.27 && !http2legacy && badlinkname

package http

import (
	_ "net/http"
	_ "unsafe"
)

const extendedConnectAvailable = true

//go:linkname disableExtendedConnectProtocol net/http/internal/http2.disableExtendedConnectProtocol
var disableExtendedConnectProtocol bool

func init() {
	disableExtendedConnectProtocol = false
}
