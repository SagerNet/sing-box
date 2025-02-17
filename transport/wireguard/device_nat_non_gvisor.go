//go:build !with_gvisor

package wireguard

type gVisorOutbound struct{}

func newGVisorOutbound() gVisorOutbound {
	return gVisorOutbound{}
}

func (d *natDeviceWrapper) Read(bufs [][]byte, sizes []int, offset int) (n int, err error) {
	select {
	case packet := <-d.packetOutbound:
		defer packet.Release()
		sizes[0] = copy(bufs[0][offset:], packet.Bytes())
		return 1, nil
	default:
	}
	return d.Device.Read(bufs, sizes, offset)
}
