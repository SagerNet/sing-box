//go:build !go1.21

package dialer

import (
	"net"
)

const multipathTCPAvailable = false

func setMultiPathTCP(dialer *net.Dialer) {
}
