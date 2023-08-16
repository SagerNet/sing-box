//go:build go1.21

package inbound

import "net"

const go121Available = true

func setMultiPathTCP(listenConfig *net.ListenConfig) {
	listenConfig.SetMultipathTCP(true)
}
