//go:build !linux

package openvpn

import "github.com/sagernet/sing/common"

const openVPNUDPSocketBufferSize = 7 << 20

type openVPNUDPSocketBufferSetter interface {
	SetReadBuffer(bytes int) error
	SetWriteBuffer(bytes int) error
}

func tuneOpenVPNUDPSocket(connection any) {
	bufferSetter, loaded := common.Cast[openVPNUDPSocketBufferSetter](connection)
	if !loaded {
		return
	}
	_ = bufferSetter.SetReadBuffer(openVPNUDPSocketBufferSize)
	_ = bufferSetter.SetWriteBuffer(openVPNUDPSocketBufferSize)
}
