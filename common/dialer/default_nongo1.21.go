//go:build !go1.21

package dialer

import (
	"net"
)

const go121Available = false

func setMultiPathTCP(dialer *net.Dialer) {
}
