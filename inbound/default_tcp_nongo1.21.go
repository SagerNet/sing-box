//go:build !go1.21

package inbound

import "net"

const go121Available = false

func setMultiPathTCP(listenConfig *net.ListenConfig) {
}
