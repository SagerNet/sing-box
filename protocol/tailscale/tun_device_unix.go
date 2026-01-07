//go:build !windows

package tailscale

import (
	"encoding/hex"
	"errors"
	"io"
	"os"
	"sync"
	"sync/atomic"

	singTun "github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/logger"
	wgTun "github.com/sagernet/wireguard-go/tun"
)

type tunDeviceAdapter struct {
	tun        singTun.Tun
	linuxTUN   singTun.LinuxTUN
	events     chan wgTun.Event
	mtu        int
	logger     logger.ContextLogger
	debugTun   bool
	readCount  atomic.Uint32
	writeCount atomic.Uint32
	closeOnce  sync.Once
}

func newTunDeviceAdapter(tun singTun.Tun, mtu int, logger logger.ContextLogger) (wgTun.Device, error) {
	if tun == nil {
		return nil, os.ErrInvalid
	}
	if mtu == 0 {
		mtu = 1500
	}
	adapter := &tunDeviceAdapter{
		tun:      tun,
		events:   make(chan wgTun.Event, 1),
		mtu:      mtu,
		logger:   logger,
		debugTun: os.Getenv("SINGBOX_TS_TUN_DEBUG") != "",
	}
	if linuxTUN, ok := tun.(singTun.LinuxTUN); ok {
		adapter.linuxTUN = linuxTUN
	}
	adapter.events <- wgTun.EventUp
	return adapter, nil
}

func (a *tunDeviceAdapter) File() *os.File {
	return nil
}

func (a *tunDeviceAdapter) Read(bufs [][]byte, sizes []int, offset int) (count int, err error) {
	if a.linuxTUN != nil {
		n, err := a.linuxTUN.BatchRead(bufs, offset-singTun.PacketOffset, sizes)
		if err == nil {
			for i := 0; i < n; i++ {
				a.debugPacket("read", bufs[i][offset:offset+sizes[i]])
			}
		}
		return n, err
	}
	if offset < singTun.PacketOffset {
		return 0, io.ErrShortBuffer
	}
	readBuf := bufs[0][offset-singTun.PacketOffset:]
	n, err := a.tun.Read(readBuf)
	if err == nil {
		if n < singTun.PacketOffset {
			return 0, io.ErrUnexpectedEOF
		}
		sizes[0] = n - singTun.PacketOffset
		a.debugPacket("read", readBuf[singTun.PacketOffset:n])
		return 1, nil
	}
	if errors.Is(err, singTun.ErrTooManySegments) {
		err = wgTun.ErrTooManySegments
	}
	return 0, err
}

func (a *tunDeviceAdapter) Write(bufs [][]byte, offset int) (count int, err error) {
	if a.linuxTUN != nil {
		for i := range bufs {
			a.debugPacket("write", bufs[i][offset:])
		}
		return a.linuxTUN.BatchWrite(bufs, offset)
	}
	for _, packet := range bufs {
		a.debugPacket("write", packet[offset:])
		if singTun.PacketOffset > 0 {
			common.ClearArray(packet[offset-singTun.PacketOffset : offset])
			singTun.PacketFillHeader(packet[offset-singTun.PacketOffset:], singTun.PacketIPVersion(packet[offset:]))
		}
		_, err = a.tun.Write(packet[offset-singTun.PacketOffset:])
		if err != nil {
			return 0, err
		}
	}
	// WireGuard will not read count.
	return 0, nil
}

func (a *tunDeviceAdapter) MTU() (int, error) {
	return a.mtu, nil
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
	if a.linuxTUN != nil {
		return a.linuxTUN.BatchSize()
	}
	return 1
}

func (a *tunDeviceAdapter) debugPacket(direction string, packet []byte) {
	if !a.debugTun || a.logger == nil {
		return
	}
	var counter *atomic.Uint32
	switch direction {
	case "read":
		counter = &a.readCount
	case "write":
		counter = &a.writeCount
	default:
		return
	}
	if counter.Add(1) > 8 {
		return
	}
	sample := packet
	if len(sample) > 64 {
		sample = sample[:64]
	}
	a.logger.Trace("tailscale tun ", direction, " len=", len(packet), " head=", hex.EncodeToString(sample))
}
