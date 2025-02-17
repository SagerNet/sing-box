package wireguard

import (
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/buf"
)

var _ Device = (*natDeviceWrapper)(nil)

type natDeviceWrapper struct {
	Device
	gVisorOutbound
	packetOutbound chan *buf.Buffer
	mapping        *tun.NatMapping
	writer         *tun.NatWriter
	buffer         [][]byte
}

func NewNATDevice(upstream Device, ipRewrite bool) NatDevice {
	wrapper := &natDeviceWrapper{
		Device:         upstream,
		gVisorOutbound: newGVisorOutbound(),
		packetOutbound: make(chan *buf.Buffer, 256),
		mapping:        tun.NewNatMapping(ipRewrite),
	}
	if ipRewrite {
		wrapper.writer = tun.NewNatWriter(upstream.Inet4Address(), upstream.Inet6Address())
	}
	return wrapper
}

func (d *natDeviceWrapper) Write(bufs [][]byte, offset int) (int, error) {
	for _, buffer := range bufs {
		handled, err := d.mapping.WritePacket(buffer[offset:])
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
	d.mapping.CreateSession(session, routeContext)
	return &natDestinationWrapper{d, session}, nil
}

var _ tun.DirectRouteDestination = (*natDestinationWrapper)(nil)

type natDestinationWrapper struct {
	device  *natDeviceWrapper
	session tun.DirectRouteSession
}

func (d *natDestinationWrapper) WritePacket(buffer *buf.Buffer) error {
	if d.device.writer != nil {
		d.device.writer.RewritePacket(buffer.Bytes())
	}
	d.device.packetOutbound <- buffer
	return nil
}

func (d *natDestinationWrapper) Close() error {
	d.device.mapping.DeleteSession(d.session)
	return nil
}

func (d *natDestinationWrapper) Timeout() bool {
	return false
}
