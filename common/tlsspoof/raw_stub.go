//go:build !linux && !darwin && !(windows && (amd64 || 386))

package tlsspoof

import (
	"net"

	E "github.com/sagernet/sing/common/exceptions"
)

const PlatformSupported = false

func newRawSpoofer(conn net.Conn, method Method) (rawSpoofer, error) {
	return nil, E.New("tls_spoof: unsupported platform")
}
