package http

import (
	_ "unsafe"

	_ "golang.org/x/net/http2"
)

//go:linkname disableExtendedConnectProtocol golang.org/x/net/http2.disableExtendedConnectProtocol
var disableExtendedConnectProtocol bool

func init() {
	disableExtendedConnectProtocol = false
}
