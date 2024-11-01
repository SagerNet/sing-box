//go:build go1.21

package listener

import "net"

const go121Available = true

func setMultiPathTCP(listenConfig *net.ListenConfig) {
	listenConfig.SetMultipathTCP(true)
}
