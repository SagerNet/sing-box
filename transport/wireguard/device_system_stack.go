package wireguard

import (
	"net/netip"
	"sync"
	"sync/atomic"

	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing-tun/gtcpip/header"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/wireguard-go/device"
)

var _ Device = (*systemStackDevice)(nil)

type systemStackDevice struct {
	*systemDevice
	stack     *tun.Go
	memoryTun *tun.MemoryTun
	device    atomic.Pointer[device.Device]
	closeOnce sync.Once
}

func newSystemStackDevice(options DeviceOptions) (*systemStackDevice, error) {
	system, err := newSystemDevice(options)
	if err != nil {
		return nil, err
	}
	stackDevice := &systemStackDevice{systemDevice: system}
	stackDevice.memoryTun = tun.NewMemoryTun(tun.MemoryTunOptions{MTU: int(options.MTU), Outbound: stackDevice.inputPackets})
	stackDevice.stack, err = newStack(options, stackDevice.memoryTun)
	if err != nil {
		return nil, err
	}
	return stackDevice, nil
}

func (w *systemStackDevice) SetDevice(device *device.Device) {
	w.device.Store(device)
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
		_, err := w.memoryTun.WritePackets(stackPackets)
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

func (w *systemStackDevice) inputPackets(packets []*buf.Buffer) error {
	defer buf.ReleaseMulti(packets)
	wgDevice := w.device.Load()
	if wgDevice == nil {
		return nil
	}
	references := make([]*device.InputPacketRef, 0, len(packets))
	for _, packet := range packets {
		var destination []byte
		switch header.IPVersion(packet.Bytes()) {
		case header.IPv4Version:
			destination = header.IPv4(packet.Bytes()).DestinationAddressSlice()
		case header.IPv6Version:
			destination = header.IPv6(packet.Bytes()).DestinationAddressSlice()
		default:
			continue
		}
		references = append(references, &device.InputPacketRef{Destination: destination, PacketSlices: [][]byte{packet.Bytes()}})
	}
	wgDevice.InputPackets(references)
	return nil
}

func (w *systemStackDevice) Close() error {
	var err error
	w.closeOnce.Do(func() {
		err = E.Errors(w.stack.Close(), w.memoryTun.Close(), w.systemDevice.Close())
	})
	return err
}
