//go:build with_gvisor

package wireguard

import (
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"

	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

func (d *natDestinationWrapper) WritePacketBuffer(buffer *stack.PacketBuffer) error {
	defer buffer.DecRef()
	if d.device.writer != nil {
		d.device.writer.RewritePacketBuffer(buffer)
	}
	var packetLen int
	for _, slice := range buffer.AsSlices() {
		packetLen += len(slice)
	}
	packet := buf.NewSize(packetLen)
	for _, slice := range buffer.AsSlices() {
		common.Must1(packet.Write(slice))
	}
	d.device.outbound <- packet
	return nil
}
