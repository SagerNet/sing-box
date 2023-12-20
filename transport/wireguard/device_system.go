package wireguard

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	wgTun "github.com/sagernet/wireguard-go/tun"
)

var _ Device = (*SystemDevice)(nil)

type SystemDevice struct {
	dialer      N.Dialer
	device      tun.Tun
	batchDevice tun.LinuxTUN
	name        string
	mtu         int
	events      chan wgTun.Event
	addr4       netip.Addr
	addr6       netip.Addr
}

func NewSystemDevice(router adapter.Router, interfaceName string, localPrefixes []netip.Prefix, mtu uint32, gso bool) (*SystemDevice, error) {
	var inet4Addresses []netip.Prefix
	var inet6Addresses []netip.Prefix
	for _, prefixes := range localPrefixes {
		if prefixes.Addr().Is4() {
			inet4Addresses = append(inet4Addresses, prefixes)
		} else {
			inet6Addresses = append(inet6Addresses, prefixes)
		}
	}
	if interfaceName == "" {
		interfaceName = tun.CalculateInterfaceName("wg")
	}
	tunInterface, err := tun.New(tun.Options{
		Name:         interfaceName,
		Inet4Address: inet4Addresses,
		Inet6Address: inet6Addresses,
		MTU:          mtu,
		GSO:          gso,
	})
	if err != nil {
		return nil, err
	}
	var inet4Address netip.Addr
	var inet6Address netip.Addr
	if len(inet4Addresses) > 0 {
		inet4Address = inet4Addresses[0].Addr()
	}
	if len(inet6Addresses) > 0 {
		inet6Address = inet6Addresses[0].Addr()
	}
	var batchDevice tun.LinuxTUN
	if gso {
		batchTUN, isBatchTUN := tunInterface.(tun.LinuxTUN)
		if !isBatchTUN {
			return nil, E.New("GSO is not supported on current platform")
		}
		batchDevice = batchTUN
	}
	return &SystemDevice{
		dialer: common.Must1(dialer.NewDefault(router, option.DialerOptions{
			BindInterface: interfaceName,
		})),
		device:      tunInterface,
		batchDevice: batchDevice,
		name:        interfaceName,
		mtu:         int(mtu),
		events:      make(chan wgTun.Event),
		addr4:       inet4Address,
		addr6:       inet6Address,
	}, nil
}

func (w *SystemDevice) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return w.dialer.DialContext(ctx, network, destination)
}

func (w *SystemDevice) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return w.dialer.ListenPacket(ctx, destination)
}

func (w *SystemDevice) Inet4Address() netip.Addr {
	return w.addr4
}

func (w *SystemDevice) Inet6Address() netip.Addr {
	return w.addr6
}

func (w *SystemDevice) Start() error {
	w.events <- wgTun.EventUp
	return nil
}

func (w *SystemDevice) File() *os.File {
	return nil
}

func (w *SystemDevice) Read(bufs [][]byte, sizes []int, offset int) (count int, err error) {
	if w.batchDevice != nil {
		count, err = w.batchDevice.BatchRead(bufs, offset, sizes)
	} else {
		sizes[0], err = w.device.Read(bufs[0][offset:])
		if err == nil {
			count = 1
		} else if errors.Is(err, tun.ErrTooManySegments) {
			err = wgTun.ErrTooManySegments
		}
	}
	return
}

func (w *SystemDevice) Write(bufs [][]byte, offset int) (count int, err error) {
	if w.batchDevice != nil {
		return 0, w.batchDevice.BatchWrite(bufs, offset)
	} else {
		for _, b := range bufs {
			_, err = w.device.Write(b[offset:])
			if err != nil {
				return
			}
		}
	}
	// WireGuard will not read count
	return
}

func (w *SystemDevice) Flush() error {
	return nil
}

func (w *SystemDevice) MTU() (int, error) {
	return w.mtu, nil
}

func (w *SystemDevice) Name() (string, error) {
	return w.name, nil
}

func (w *SystemDevice) Events() <-chan wgTun.Event {
	return w.events
}

func (w *SystemDevice) Close() error {
	return w.device.Close()
}

func (w *SystemDevice) BatchSize() int {
	if w.batchDevice != nil {
		return w.batchDevice.BatchSize()
	}
	return 1
}
