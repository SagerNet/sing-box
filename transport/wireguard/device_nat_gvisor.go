//go:build with_gvisor

package wireguard

import (
	"github.com/sagernet/gvisor/pkg/tcpip/stack"
)

type gVisorOutbound struct {
	outbound chan *stack.PacketBuffer
}

func newGVisorOutbound() gVisorOutbound {
	return gVisorOutbound{
		outbound: make(chan *stack.PacketBuffer, 256),
	}
}

func (d *natDeviceWrapper) Read(bufs [][]byte, sizes []int, offset int) (n int, err error) {
	select {
	case packet := <-d.outbound:
		defer packet.DecRef()
		var copyN int
		/*rangeIterate(packet.Data().AsRange(), func(view *buffer.View) {
			copyN += copy(bufs[0][offset+copyN:], view.AsSlice())
		})*/
		for _, view := range packet.AsSlices() {
			copyN += copy(bufs[0][offset+copyN:], view)
		}
		sizes[0] = copyN
		return 1, nil
	case packet := <-d.packetOutbound:
		defer packet.Release()
		sizes[0] = copy(bufs[0][offset:], packet.Bytes())
		return 1, nil
	default:
	}
	return d.Device.Read(bufs, sizes, offset)
}

func (d *natDestinationWrapper) WritePacketBuffer(packetBuffer *stack.PacketBuffer) error {
	println("read from wg")
	if d.device.writer != nil {
		d.device.writer.RewritePacketBuffer(packetBuffer)
	}
	d.device.outbound <- packetBuffer
	return nil
}
