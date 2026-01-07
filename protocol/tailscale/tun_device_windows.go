//go:build windows

package tailscale

import (
	"errors"
	"os"
	"sync"
	"sync/atomic"

	singTun "github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/logger"
	wgTun "github.com/sagernet/wireguard-go/tun"
)

type tunDeviceAdapter struct {
	tun       singTun.WinTun
	nativeTun *singTun.NativeTun
	events    chan wgTun.Event
	mtu       atomic.Int64
	closeOnce sync.Once
}

func newTunDeviceAdapter(tun singTun.Tun, mtu int, _ logger.ContextLogger) (wgTun.Device, error) {
	winTun, ok := tun.(singTun.WinTun)
	if !ok {
		return nil, errors.New("not a windows tun device")
	}
	nativeTun, ok := winTun.(*singTun.NativeTun)
	if !ok {
		return nil, errors.New("unsupported windows tun device")
	}
	if mtu == 0 {
		mtu = 1500
	}
	adapter := &tunDeviceAdapter{
		tun:       winTun,
		nativeTun: nativeTun,
		events:    make(chan wgTun.Event, 1),
	}
	adapter.mtu.Store(int64(mtu))
	adapter.events <- wgTun.EventUp
	return adapter, nil
}

func (a *tunDeviceAdapter) File() *os.File {
	return nil
}

func (a *tunDeviceAdapter) Read(bufs [][]byte, sizes []int, offset int) (count int, err error) {
	packet, release, err := a.tun.ReadPacket()
	if err != nil {
		return 0, err
	}
	defer release()
	sizes[0] = copy(bufs[0][offset-singTun.PacketOffset:], packet)
	return 1, nil
}

func (a *tunDeviceAdapter) Write(bufs [][]byte, offset int) (count int, err error) {
	for _, packet := range bufs {
		if singTun.PacketOffset > 0 {
			singTun.PacketFillHeader(packet[offset-singTun.PacketOffset:], singTun.PacketIPVersion(packet[offset:]))
		}
		_, err = a.tun.Write(packet[offset-singTun.PacketOffset:])
		if err != nil {
			return 0, err
		}
	}
	return 0, nil
}

func (a *tunDeviceAdapter) MTU() (int, error) {
	return int(a.mtu.Load()), nil
}

func (a *tunDeviceAdapter) ForceMTU(mtu int) {
	if mtu <= 0 {
		return
	}
	update := int(a.mtu.Load()) != mtu
	a.mtu.Store(int64(mtu))
	if update {
		select {
		case a.events <- wgTun.EventMTUUpdate:
		default:
		}
	}
}

func (a *tunDeviceAdapter) LUID() uint64 {
	if a.nativeTun == nil {
		return 0
	}
	return a.nativeTun.LUID()
}

func (a *tunDeviceAdapter) Name() (string, error) {
	return a.tun.Name()
}

func (a *tunDeviceAdapter) Events() <-chan wgTun.Event {
	return a.events
}

func (a *tunDeviceAdapter) Close() error {
	var err error
	a.closeOnce.Do(func() {
		close(a.events)
		err = a.tun.Close()
	})
	return err
}

func (a *tunDeviceAdapter) BatchSize() int {
	return 1
}
