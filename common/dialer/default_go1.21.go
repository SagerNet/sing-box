//go:build go1.21

package dialer

import "net"

const go121Available = true

func setMultiPathTCP(dialer *net.Dialer) {
	dialer.SetMultipathTCP(true)
}
