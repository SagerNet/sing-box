//go:build !go1.21

package inbound

import "net"

const multipathTCPAvailable = false

func setMultiPathTCP(listenConfig *net.ListenConfig) {
}
