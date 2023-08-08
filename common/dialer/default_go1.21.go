//go:build go1.21

package dialer

import "net"

const multipathTCPAvailable = true

func setMultiPathTCP(dialer *net.Dialer) {
	dialer.SetMultipathTCP(true)
}
