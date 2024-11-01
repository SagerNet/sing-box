//go:build !go1.21

package listener

import "net"

const go121Available = false

func setMultiPathTCP(listenConfig *net.ListenConfig) {
}
