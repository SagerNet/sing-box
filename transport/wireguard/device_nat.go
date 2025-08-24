package wireguard

import (
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing-tun/ping"
	"github.com/sagernet/sing/common/buf"
)

var _ Device = (*natDeviceWrapper)(nil)

type natDeviceWrapper struct {
	Device
	packetOutbound chan *buf.Buffer
	rewriter       *ping.Rewriter
	buffer         [][]byte
}

func NewNATDevice(upstream Device) NatDevice {
	wrapper := &natDeviceWrapper{
		Device:         upstream,
		packetOutbound: make(chan *buf.Buffer, 256),
		rewriter:       ping.NewRewriter(upstream.Inet4Address(), upstream.Inet6Address()),
	}
	return wrapper
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

func (d *natDeviceWrapper) Write(bufs [][]byte, offset int) (int, error) {
	for _, buffer := range bufs {
		handled, err := d.rewriter.WriteBack(buffer[offset:])
		if handled {
			if err != nil {
				return 0, err
			}
		} else {
			d.buffer = append(d.buffer, buffer)
		}
	}
	if len(d.buffer) > 0 {
		_, err := d.Device.Write(d.buffer, offset)
		if err != nil {
			return 0, err
		}
		d.buffer = d.buffer[:0]
	}
	return 0, nil
}

func (d *natDeviceWrapper) CreateDestination(metadata adapter.InboundContext, routeContext tun.DirectRouteContext) (tun.DirectRouteDestination, error) {
	session := tun.DirectRouteSession{
		Source:      metadata.Source.Addr,
		Destination: metadata.Destination.Addr,
	}
	d.rewriter.CreateSession(session, routeContext)
	return &natDestination{d, session}, nil
}

var _ tun.DirectRouteDestination = (*natDestination)(nil)

type natDestination struct {
	device  *natDeviceWrapper
	session tun.DirectRouteSession
}

func (d *natDestination) WritePacket(buffer *buf.Buffer) error {
	d.device.rewriter.RewritePacket(buffer.Bytes())
	d.device.packetOutbound <- buffer
	return nil
}

func (d *natDestination) Close() error {
	d.device.rewriter.DeleteSession(d.session)
	return nil
}
