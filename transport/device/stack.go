package device

import (
	"context"
	"net"
	"net/netip"
	"sync"

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
	closeOnce    sync.Once
}

func newStackDevice(options Options) (*stackDevice, error) {
	device := &stackDevice{
		baseDevice: baseDevice{packetHeadroom: options.PacketHeadroom},
		options:    options,
	}
	device.inet4Address, device.inet6Address = firstAddresses(options.Configuration.Address)
	device.memoryTun = tun.NewMemoryTun(tun.MemoryTunOptions{
		MTU:       int(options.MTU),
		Headroom:  options.PacketHeadroom,
		RearSpace: systemDevicePacketRearSpace,
		Outbound:  device.writeOutbound,
	})
	var err error
	device.stack, err = newStack(options, device.memoryTun)
	if err != nil {
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
	d.inet4Address, d.inet6Address = firstAddresses(configuration.Address)
	return nil
}

func (d *stackDevice) WriteInboundBuffers(packetBuffers []*buf.Buffer) error {
	return d.processInboundBuffers(packetBuffers, d.writeBuffers)
}

func (d *stackDevice) writeBuffers(packetBuffers []*buf.Buffer) error {
	_, err := d.memoryTun.WritePackets(common.Map(packetBuffers, (*buf.Buffer).Bytes))
	return err
}

func (d *stackDevice) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	bind, err := d.bindAddress(destination)
	if err != nil {
		return nil, err
	}
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		var tcpConn *tun.GoConn
		tcpConn, err = d.stack.DialTCP(ctx, bind, destination.AddrPort())
		if err != nil {
			return nil, err
		}
		return tcpConn, nil
	case N.NetworkUDP:
		var udpConn *tun.GoUDPConn
		udpConn, err = d.stack.DialUDP(netip.AddrPortFrom(bind, 0), destination.AddrPort())
		if err != nil {
			return nil, err
		}
		return udpConn, nil
	default:
		return nil, E.Extend(N.ErrUnknownNetwork, network)
	}
}

func (d *stackDevice) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	bind, err := d.bindAddress(destination)
	if err != nil {
		return nil, err
	}
	udpConn, err := d.stack.ListenUDP(netip.AddrPortFrom(bind, 0))
	if err != nil {
		return nil, err
	}
	return udpConn, nil
}

func (d *stackDevice) bindAddress(destination M.Socksaddr) (netip.Addr, error) {
	d.stateAccess.RLock()
	defer d.stateAccess.RUnlock()
	if destination.IsIPv4() {
		if !d.inet4Address.IsValid() {
			return netip.Addr{}, E.New("missing IPv4 local address")
		}
		return d.inet4Address, nil
	}
	if d.options.Configuration.BlockIPv6 {
		return netip.Addr{}, E.New("IPv6 is blocked")
	}
	if !d.inet6Address.IsValid() {
		return netip.Addr{}, E.New("missing IPv6 local address")
	}
	return d.inet6Address, nil
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

func (d *stackDevice) Close() error {
	var err error
	d.closeOnce.Do(func() {
		err = E.Errors(d.stack.Close(), d.memoryTun.Close())
	})
	return err
}
