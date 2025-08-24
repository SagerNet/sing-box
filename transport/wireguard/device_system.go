package wireguard

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"runtime"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/wireguard-go/device"
	wgTun "github.com/sagernet/wireguard-go/tun"
)

var _ Device = (*systemDevice)(nil)

type systemDevice struct {
	options      DeviceOptions
	dialer       N.Dialer
	device       tun.Tun
	batchDevice  tun.LinuxTUN
	events       chan wgTun.Event
	closeOnce    sync.Once
	inet4Address netip.Addr
	inet6Address netip.Addr
}

func newSystemDevice(options DeviceOptions) (*systemDevice, error) {
	if options.Name == "" {
		options.Name = tun.CalculateInterfaceName("wg")
	}
	var inet4Address netip.Addr
	var inet6Address netip.Addr
	if len(options.Address) > 0 {
		if prefix := common.Find(options.Address, func(it netip.Prefix) bool {
			return it.Addr().Is4()
		}); prefix.IsValid() {
			inet4Address = prefix.Addr()
		}
	}
	if len(options.Address) > 0 {
		if prefix := common.Find(options.Address, func(it netip.Prefix) bool {
			return it.Addr().Is6()
		}); prefix.IsValid() {
			inet6Address = prefix.Addr()
		}
	}
	return &systemDevice{
		options:      options,
		dialer:       options.CreateDialer(options.Name),
		events:       make(chan wgTun.Event, 1),
		inet4Address: inet4Address,
		inet6Address: inet6Address,
	}, nil
}

func (w *systemDevice) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return w.dialer.DialContext(ctx, network, destination)
}

func (w *systemDevice) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return w.dialer.ListenPacket(ctx, destination)
}

func (w *systemDevice) Inet4Address() netip.Addr {
	return w.inet4Address
}

func (w *systemDevice) Inet6Address() netip.Addr {
	return w.inet6Address
}

func (w *systemDevice) SetDevice(device *device.Device) {
}

func (w *systemDevice) Start() error {
	networkManager := service.FromContext[adapter.NetworkManager](w.options.Context)
	tunOptions := tun.Options{
		Name: w.options.Name,
		Inet4Address: common.Filter(w.options.Address, func(it netip.Prefix) bool {
			return it.Addr().Is4()
		}),
		Inet6Address: common.Filter(w.options.Address, func(it netip.Prefix) bool {
			return it.Addr().Is6()
		}),
		MTU:            w.options.MTU,
		GSO:            true,
		InterfaceScope: true,
		Inet4RouteAddress: common.Filter(w.options.AllowedAddress, func(it netip.Prefix) bool {
			return it.Addr().Is4()
		}),
		Inet6RouteAddress: common.Filter(w.options.AllowedAddress, func(it netip.Prefix) bool { return it.Addr().Is6() }),
		InterfaceMonitor:  networkManager.InterfaceMonitor(),
		InterfaceFinder:   networkManager.InterfaceFinder(),
		Logger:            w.options.Logger,
	}
	// works with Linux, macOS with IFSCOPE routes, not tested on Windows
	if runtime.GOOS == "darwin" {
		tunOptions.AutoRoute = true
	}
	tunInterface, err := tun.New(tunOptions)
	if err != nil {
		return err
	}
	err = tunInterface.Start()
	if err != nil {
		return err
	}
	w.options.Logger.Info("started at ", w.options.Name)
	w.device = tunInterface
	batchTUN, isBatchTUN := tunInterface.(tun.LinuxTUN)
	if isBatchTUN {
		w.batchDevice = batchTUN
	}
	w.events <- wgTun.EventUp
	return nil
}

func (w *systemDevice) File() *os.File {
	return nil
}

func (w *systemDevice) Read(bufs [][]byte, sizes []int, offset int) (count int, err error) {
	if w.batchDevice != nil {
		count, err = w.batchDevice.BatchRead(bufs, offset-tun.PacketOffset, sizes)
	} else {
		sizes[0], err = w.device.Read(bufs[0][offset-tun.PacketOffset:])
		if err == nil {
			count = 1
		} else if errors.Is(err, tun.ErrTooManySegments) {
			err = wgTun.ErrTooManySegments
		}
	}
	return
}

func (w *systemDevice) Write(bufs [][]byte, offset int) (count int, err error) {
	if w.batchDevice != nil {
		return w.batchDevice.BatchWrite(bufs, offset)
	} else {
		for _, packet := range bufs {
			if tun.PacketOffset > 0 {
				common.ClearArray(packet[offset-tun.PacketOffset : offset])
				tun.PacketFillHeader(packet[offset-tun.PacketOffset:], tun.PacketIPVersion(packet[offset:]))
			}
			_, err = w.device.Write(packet[offset-tun.PacketOffset:])
			if err != nil {
				return
			}
		}
	}
	// WireGuard will not read count
	return
}

func (w *systemDevice) Flush() error {
	return nil
}

func (w *systemDevice) MTU() (int, error) {
	return int(w.options.MTU), nil
}

func (w *systemDevice) Name() (string, error) {
	return w.options.Name, nil
}

func (w *systemDevice) Events() <-chan wgTun.Event {
	return w.events
}

func (w *systemDevice) Close() error {
	close(w.events)
	return w.device.Close()
}

func (w *systemDevice) BatchSize() int {
	if w.batchDevice != nil {
		return w.batchDevice.BatchSize()
	}
	return 1
}
