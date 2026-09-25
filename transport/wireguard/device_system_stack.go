package wireguard

import (
	"net/netip"
	"sync"

	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing-tun/gtcpip/header"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/wireguard-go/device"
)

var _ Device = (*systemStackDevice)(nil)

type systemStackDevice struct {
	*systemDevice
	stack     *tun.Go
	router    *peerRouter
	closeOnce sync.Once
}

func newSystemStackDevice(options DeviceOptions) (*systemStackDevice, error) {
	system, err := newSystemDevice(options)
	if err != nil {
		return nil, err
	}
	router := newPeerRouter(options)
	stack, err := newStack(options, router.memoryTun)
	if err != nil {
		router.close()
		return nil, err
	}
	return &systemStackDevice{
		systemDevice: system,
		stack:        stack,
		router:       router,
	}, nil
}

func (w *systemStackDevice) SetDevice(device *device.Device, peers []*device.Peer) {
	w.router.setPeers(device.AllowedIPs(), peers)
}

func (w *systemStackDevice) Start() error {
	err := w.stack.Start()
	if err != nil {
		return err
	}
	err = w.systemDevice.Start()
	if err != nil {
		w.stack.Close()
	}
	return err
}

func (w *systemStackDevice) Write(bufs [][]byte, offset int) (int, error) {
	stackPackets := make([][]byte, 0, len(bufs))
	systemPackets := make([][]byte, 0, len(bufs))
	for _, packet := range bufs {
		if w.isLocalDestination(packet[offset:]) {
			systemPackets = append(systemPackets, packet)
		} else {
			stackPackets = append(stackPackets, packet[offset:])
		}
	}
	if len(stackPackets) > 0 {
		_, err := w.router.memoryTun.WritePackets(stackPackets)
		if err != nil {
			return 0, err
		}
	}
	if len(systemPackets) > 0 {
		return w.systemDevice.Write(systemPackets, offset)
	}
	return len(bufs), nil
}

func (w *systemStackDevice) isLocalDestination(packet []byte) bool {
	var destination netip.Addr
	switch header.IPVersion(packet) {
	case header.IPv4Version:
		destination = header.IPv4(packet).DestinationAddr()
	case header.IPv6Version:
		destination = header.IPv6(packet).DestinationAddr()
	default:
		return true
	}
	for _, prefix := range w.options.Address {
		if prefix.Contains(destination) {
			return true
		}
	}
	return false
}

func (w *systemStackDevice) Close() error {
	var err error
	w.closeOnce.Do(func() {
		err = E.Errors(w.stack.Close(), w.router.close(), w.systemDevice.Close())
	})
	return err
}
