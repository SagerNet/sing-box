package wireguard

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing-tun/ping"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/logger"
)

var _ Device = (*natDeviceWrapper)(nil)

type natDeviceWrapper struct {
	Device
	ctx            context.Context
	logger         logger.ContextLogger
	packetOutbound chan *buf.Buffer
	rewriter       *ping.SourceRewriter
	buffer         [][]byte
}

func NewNATDevice(ctx context.Context, logger logger.ContextLogger, upstream Device) NatDevice {
	wrapper := &natDeviceWrapper{
		Device:         upstream,
		ctx:            ctx,
		logger:         logger,
		packetOutbound: make(chan *buf.Buffer, 256),
		rewriter:       ping.NewSourceRewriter(ctx, logger, upstream.Inet4Address(), upstream.Inet6Address()),
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

func (d *natDeviceWrapper) CreateDestination(metadata adapter.InboundContext, routeContext tun.DirectRouteContext, timeout time.Duration) (tun.DirectRouteDestination, error) {
	ctx := log.ContextWithNewID(d.ctx)
	session := tun.DirectRouteSession{
		Source:      metadata.Source.Addr,
		Destination: metadata.Destination.Addr,
	}
	d.rewriter.CreateSession(session, routeContext)
	d.logger.InfoContext(ctx, "linked ", metadata.Network, " connection from ", metadata.Source.AddrString(), " to ", metadata.Destination.AddrString())
	return &natDestination{device: d, session: session}, nil
}

var _ tun.DirectRouteDestination = (*natDestination)(nil)

type natDestination struct {
	device  *natDeviceWrapper
	session tun.DirectRouteSession
	closed  atomic.Bool
}

func (d *natDestination) WritePacket(buffer *buf.Buffer) error {
	d.device.rewriter.RewritePacket(buffer.Bytes())
	d.device.packetOutbound <- buffer
	return nil
}

func (d *natDestination) Close() error {
	d.closed.Store(true)
	d.device.rewriter.DeleteSession(d.session)
	return nil
}

func (d *natDestination) IsClosed() bool {
	return d.closed.Load()
}
