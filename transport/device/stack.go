package device

import (
	"context"
	"net"
	"net/netip"
	"sync"

	"github.com/sagernet/sing-box/common/interrupt"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ Device = (*stackDevice)(nil)

type stackDevice struct {
	baseDevice
	stateAccess  sync.RWMutex
	options      Options
	stack        *tun.Go
	memoryTun    *tun.MemoryTun
	inet4Address netip.Addr
	inet6Address netip.Addr
	inet4UDP     interrupt.Group
	inet6UDP     interrupt.Group
	closeOnce    sync.Once
}

func newStackDevice(options Options) (*stackDevice, error) {
	device := &stackDevice{
		options: options,
	}
	device.inet4Address, device.inet6Address = firstAddresses(options.Configuration.Address)
	device.memoryTun = tun.NewMemoryTun(tun.MemoryTunOptions{
		MTU:       int(options.MTU),
		Headroom:  options.PacketFrontHeadroom,
		RearSpace: options.PacketRearHeadroom,
		Outbound: func(packetBuffers []*buf.Buffer) {
			err := device.writeOutbound(packetBuffers)
			if err != nil {
				options.Logger.Debug(E.Cause(err, "write packet batch"))
			}
		},
		Route: options.Route,
	})
	var err error
	device.stack, err = newStack(options, device.memoryTun)
	if err != nil {
		device.memoryTun.Close()
		return nil, err
	}
	return device, nil
}

func (d *stackDevice) Start() error {
	return d.stack.Start()
}

func (d *stackDevice) UpdateConfiguration(configuration Configuration) error {
	d.stateAccess.Lock()
	defer d.stateAccess.Unlock()
	if configuration.MTU != 0 {
		d.options.MTU = configuration.MTU
		d.memoryTun.UpdateMTU(int(configuration.MTU))
	}
	d.options.Configuration = configuration
	inet4Address, inet6Address := firstAddresses(configuration.Address)
	if d.inet4Address != inet4Address {
		d.inet4UDP.Interrupt(true)
	}
	if d.inet6Address != inet6Address {
		d.inet6UDP.Interrupt(true)
	}
	d.inet4Address, d.inet6Address = inet4Address, inet6Address
	return nil
}

func (d *stackDevice) WriteInboundBuffers(packetBuffers []*buf.Buffer) error {
	return d.processInboundBuffers(packetBuffers, d.writeBuffers)
}

func (d *stackDevice) WaitWriteInboundBuffers(packetBuffers []*buf.Buffer) error {
	return d.processInboundBuffers(packetBuffers, d.waitWriteBuffers)
}

func (d *stackDevice) writeBuffers(packetBuffers []*buf.Buffer) error {
	_, err := d.memoryTun.WritePackets(common.Map(packetBuffers, (*buf.Buffer).Bytes))
	return err
}

func (d *stackDevice) waitWriteBuffers(packetBuffers []*buf.Buffer) error {
	return d.memoryTun.WaitWritePackets(common.Map(packetBuffers, (*buf.Buffer).Bytes))
}

func (d *stackDevice) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		d.stateAccess.RLock()
		bind, _, err := d.bindAddressLocked(destination)
		d.stateAccess.RUnlock()
		if err != nil {
			return nil, err
		}
		tcpConn, err := d.stack.DialTCP(ctx, bind, destination.AddrPort())
		if err != nil {
			return nil, err
		}
		return tcpConn, nil
	case N.NetworkUDP:
		d.stateAccess.RLock()
		defer d.stateAccess.RUnlock()
		bind, udpGroup, err := d.bindAddressLocked(destination)
		if err != nil {
			return nil, err
		}
		udpConn, err := d.stack.DialUDP(netip.AddrPortFrom(bind, 0), destination.AddrPort())
		if err != nil {
			return nil, err
		}
		return udpGroup.NewConn(udpConn, false), nil
	default:
		return nil, E.Extend(N.ErrUnknownNetwork, network)
	}
}

func (d *stackDevice) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	d.stateAccess.RLock()
	defer d.stateAccess.RUnlock()
	bind, udpGroup, err := d.bindAddressLocked(destination)
	if err != nil {
		return nil, err
	}
	udpConn, err := d.stack.ListenUDP(netip.AddrPortFrom(bind, 0))
	if err != nil {
		return nil, err
	}
	return udpGroup.NewPacketConn(udpConn, false), nil
}

func (d *stackDevice) bindAddressLocked(destination M.Socksaddr) (netip.Addr, *interrupt.Group, error) {
	if destination.IsIPv4() {
		if !d.inet4Address.IsValid() {
			return netip.Addr{}, nil, E.New("missing IPv4 local address")
		}
		return d.inet4Address, &d.inet4UDP, nil
	}
	if d.options.Configuration.BlockIPv6 {
		return netip.Addr{}, nil, E.New("IPv6 is blocked")
	}
	if !d.inet6Address.IsValid() {
		return netip.Addr{}, nil, E.New("missing IPv6 local address")
	}
	return d.inet6Address, &d.inet6UDP, nil
}

func (d *stackDevice) PortAddresses() (netip.Addr, netip.Addr) {
	d.stateAccess.RLock()
	defer d.stateAccess.RUnlock()
	return d.inet4Address, d.inet6Address
}

func (d *stackDevice) PortMTU() uint32 {
	d.stateAccess.RLock()
	defer d.stateAccess.RUnlock()
	return d.options.MTU
}

func (d *stackDevice) UpstreamPort() any {
	return d.stack
}

func (d *stackDevice) NewOutboundQueue(handler func(packetBuffers []*buf.Buffer)) *tun.OutboundQueue {
	return d.memoryTun.NewOutboundQueue(handler)
}

func (d *stackDevice) Close() error {
	var err error
	d.closeOnce.Do(func() {
		err = E.Errors(d.stack.Close(), d.memoryTun.Close())
	})
	return err
}
